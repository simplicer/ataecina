package opencodezen_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodezen"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

type declared struct {
	items []domain.Declaration
	err   error
}

func (d declared) DeclarationsOf(domain.SubscriptionID) ([]domain.Declaration, error) {
	return d.items, d.err
}

func (declared) KeepDeclaration(domain.SubscriptionID, domain.Declaration) error { return nil }

type spending struct {
	spent domain.Money
	err   error
}

func (s spending) SpentToDate(context.Context) (domain.Money, error) { return s.spent, s.err }

func money(t *testing.T, raw, code string) domain.Money {
	t.Helper()
	amount, _ := domain.ParseAmount(raw)
	currency, _ := domain.NewCurrency(code)
	return domain.NewMoney(amount, currency)
}

var zen = domain.NewSubscription("zen", "opencode", opencodezen.SourceID, domain.FidelityDerived, "")

func read(d declared, s spending) (domain.Reading, error) {
	return opencodezen.NewSource(d, s, func() time.Time { return now }).Read(context.Background(), zen)
}

func TestTheBalanceIsDerived(t *testing.T) {
	topUp, _ := domain.NewDeclaration(domain.DeclaredTopUp, money(t, "20", "USD"), now.Add(-time.Hour), money(t, "1", "USD"))
	reading, err := read(declared{items: []domain.Declaration{topUp}}, spending{spent: money(t, "4.1", "USD")})
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	remaining, _ := balance.Remaining()
	if reading.Outcome() != domain.OutcomeOK || reading.Fidelity() != domain.FidelityDerived || remaining.Amount().String() != "16.9" {
		t.Fatalf("reading = %s %s %s", reading.Outcome(), reading.Fidelity(), remaining.Amount())
	}
}

func TestWithoutADeclarationOnlyTheSpendingIsShown(t *testing.T) {
	reading, err := read(declared{}, spending{spent: money(t, "4.1", "USD")})
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	if _, known := balance.Remaining(); known || reading.Outcome() != domain.OutcomePartial || reading.Hint() == "" {
		t.Fatalf("reading = %s %q", reading.Outcome(), reading.Hint())
	}
}

func TestFailuresAreClassified(t *testing.T) {
	down := domain.Failure(domain.ErrorUnavailable, "opencode stats did not answer", nil)
	if _, err := read(declared{}, spending{err: down}); err != down {
		t.Fatalf("spend failure = %v", err)
	}
	var failure *domain.SourceFailure
	if _, err := read(declared{err: errors.New("corrupt")}, spending{spent: money(t, "1", "USD")}); !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("ledger failure = %v", err)
	}
	inEuros, _ := domain.NewDeclaration(domain.DeclaredTopUp, money(t, "5", "EUR"), now, money(t, "0", "EUR"))
	if _, err := read(declared{items: []domain.Declaration{inEuros}}, spending{spent: money(t, "1", "USD")}); !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("currency mismatch = %v", err)
	}
}
