package application_test

import (
	"context"
	"errors"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

type ledger struct {
	declarations []domain.Declaration
	readErr      error
	keepErr      error
}

func (l *ledger) DeclarationsOf(domain.SubscriptionID) ([]domain.Declaration, error) {
	return append([]domain.Declaration(nil), l.declarations...), l.readErr
}

func (l *ledger) KeepDeclaration(_ domain.SubscriptionID, declaration domain.Declaration) error {
	if l.keepErr != nil {
		return l.keepErr
	}
	l.declarations = append(l.declarations, declaration)
	return nil
}

type spendRecord struct {
	spent domain.Money
	err   error
}

func (s spendRecord) SpentToDate(context.Context) (domain.Money, error) { return s.spent, s.err }

func money(t *testing.T, raw, code string) domain.Money {
	t.Helper()
	amount, err := domain.ParseAmount(raw)
	if err != nil {
		t.Fatal(err)
	}
	currency, err := domain.NewCurrency(code)
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewMoney(amount, currency)
}

func declareBalance(l *ledger, spend spendRecord, kind domain.DeclarationKind, amount domain.Money) (domain.Declaration, error) {
	return application.NewDeclareBalance(l, spend, fixedClock{now}).Declare(context.Background(), "zen", kind, amount)
}

func TestATopUpIsRecordedWithTheSpendingToDate(t *testing.T) {
	l := &ledger{}
	declaration, err := declareBalance(l, spendRecord{spent: money(t, "3.2", "USD")}, domain.DeclaredTopUp, money(t, "20", "USD"))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.declarations) != 1 || declaration.SpentBaseline().Amount().String() != "3.2" || !declaration.DeclaredAt().Equal(now) {
		t.Fatalf("declaration = %+v", declaration)
	}
}

func TestDeclarationsThatCannotBeTrustedAreRefused(t *testing.T) {
	usd20 := money(t, "20", "USD")
	cases := map[string]struct {
		l      *ledger
		spend  spendRecord
		kind   domain.DeclarationKind
		amount domain.Money
	}{
		"spend unknown":     {&ledger{}, spendRecord{err: errors.New("opencode missing")}, domain.DeclaredTopUp, usd20},
		"zero top-up":       {&ledger{}, spendRecord{spent: money(t, "0", "USD")}, domain.DeclaredTopUp, money(t, "0", "USD")},
		"ledger unreadable": {&ledger{readErr: errors.New("corrupt")}, spendRecord{spent: money(t, "0", "USD")}, domain.DeclaredTopUp, usd20},
		"ledger read-only":  {&ledger{keepErr: errors.New("read-only")}, spendRecord{spent: money(t, "0", "USD")}, domain.DeclaredTopUp, usd20},
	}
	for name, c := range cases {
		if _, err := declareBalance(c.l, c.spend, c.kind, c.amount); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	euros := money(t, "5", "EUR")
	inEuros, _ := domain.NewDeclaration(domain.DeclaredTopUp, euros, now, money(t, "0", "EUR"))
	mixed := &ledger{declarations: []domain.Declaration{inEuros}}
	if _, err := declareBalance(mixed, spendRecord{spent: money(t, "0", "USD")}, domain.DeclaredTopUp, usd20); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("mixed currencies = %v", err)
	}
	if len(mixed.declarations) != 1 {
		t.Fatal("a refused declaration is not kept")
	}
}
