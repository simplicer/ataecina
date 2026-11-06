package domain_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func dollars(t *testing.T, raw string) domain.Money {
	t.Helper()
	usd, _ := domain.NewCurrency("USD")
	return domain.NewMoney(mustAmount(t, raw), usd)
}

func declare(t *testing.T, kind domain.DeclarationKind, amount, baseline string, at time.Time) domain.Declaration {
	t.Helper()
	declaration, err := domain.NewDeclaration(kind, dollars(t, amount), at, dollars(t, baseline))
	if err != nil {
		t.Fatal(err)
	}
	return declaration
}

func TestTopUpsMinusWhatWasSpentSince(t *testing.T) {
	declarations := []domain.Declaration{
		declare(t, domain.DeclaredTopUp, "20", "3", noon),
		declare(t, domain.DeclaredTopUp, "10", "5", noon.Add(time.Hour)),
	}
	balance, err := domain.DeriveBalance(declarations, dollars(t, "9.5"))
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if remaining.Amount().String() != "23.5" || spent.Amount().String() != "6.5" {
		t.Fatalf("remaining %s, spent %s", remaining.Amount(), spent.Amount())
	}
}

func TestADeclaredBalanceReplacesWhatCameBefore(t *testing.T) {
	declarations := []domain.Declaration{
		declare(t, domain.DeclaredTopUp, "20", "0", noon),
		declare(t, domain.DeclaredBalance, "4", "12", noon.Add(time.Hour)),
		declare(t, domain.DeclaredTopUp, "10", "13", noon.Add(2*time.Hour)),
	}
	balance, err := domain.DeriveBalance(declarations, dollars(t, "15"))
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if remaining.Amount().String() != "11" || spent.Amount().String() != "3" {
		t.Fatalf("remaining %s, spent %s", remaining.Amount(), spent.Amount())
	}
}

func TestClearedRecordsNeverMakeSpendingNegative(t *testing.T) {
	balance, err := domain.DeriveBalance([]domain.Declaration{declare(t, domain.DeclaredBalance, "5", "40", noon)}, dollars(t, "1"))
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if remaining.Amount().String() != "5" || spent.Amount().String() != "0" {
		t.Fatalf("remaining %s, spent %s", remaining.Amount(), spent.Amount())
	}
	overspent, _ := domain.DeriveBalance([]domain.Declaration{declare(t, domain.DeclaredTopUp, "1", "0", noon)}, dollars(t, "3"))
	if left, _ := overspent.Remaining(); !left.Amount().IsNegative() {
		t.Fatal("an undeclared top-up shows as a negative balance, not hidden")
	}
}

func TestDeclarationsAreValidated(t *testing.T) {
	eur, _ := domain.NewCurrency("EUR")
	euros := domain.NewMoney(mustAmount(t, "5"), eur)
	cases := []struct {
		kind     domain.DeclarationKind
		amount   domain.Money
		baseline domain.Money
		at       time.Time
		want     error
	}{
		{"GIFT", dollars(t, "5"), dollars(t, "0"), noon, domain.ErrUnknownValue},
		{domain.DeclaredTopUp, dollars(t, "0"), dollars(t, "0"), noon, domain.ErrInvalidFigure},
		{domain.DeclaredBalance, dollars(t, "-1"), dollars(t, "0"), noon, domain.ErrInvalidFigure},
		{domain.DeclaredTopUp, euros, dollars(t, "0"), noon, domain.ErrCurrencyMismatch},
		{domain.DeclaredTopUp, dollars(t, "5"), dollars(t, "0"), time.Time{}, domain.ErrInvalidFigure},
	}
	for _, c := range cases {
		if _, err := domain.NewDeclaration(c.kind, c.amount, c.at, c.baseline); !errors.Is(err, c.want) {
			t.Fatalf("NewDeclaration(%s, %s) = %v, want %v", c.kind, c.amount.Amount(), err, c.want)
		}
	}
	zero, err := domain.NewDeclaration(domain.DeclaredBalance, dollars(t, "0"), noon, dollars(t, "0"))
	if err != nil || zero.Kind() != domain.DeclaredBalance || !zero.DeclaredAt().Equal(noon) || zero.Amount().Amount().String() != "0" || zero.SpentBaseline().Currency() != "USD" {
		t.Fatalf("an empty balance can be declared: %v", err)
	}
}

func TestDerivingNeedsDeclarationsInOneCurrency(t *testing.T) {
	if _, err := domain.DeriveBalance(nil, dollars(t, "1")); !errors.Is(err, domain.ErrNoDeclaration) {
		t.Fatalf("no declarations = %v", err)
	}
	eur, _ := domain.NewCurrency("EUR")
	euros := domain.NewMoney(mustAmount(t, "1"), eur)
	if _, err := domain.DeriveBalance([]domain.Declaration{declare(t, domain.DeclaredTopUp, "1", "0", noon)}, euros); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("spend in another currency = %v", err)
	}
	inEuros, _ := domain.NewDeclaration(domain.DeclaredTopUp, euros, noon, euros)
	if _, err := domain.DeriveBalance([]domain.Declaration{declare(t, domain.DeclaredTopUp, "1", "0", noon), inEuros}, dollars(t, "1")); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("top-ups in two currencies = %v", err)
	}
	for _, raw := range []string{"TOP_UP", "BALANCE"} {
		if kind, err := domain.ParseDeclarationKind(raw); err != nil || string(kind) != raw {
			t.Fatalf("ParseDeclarationKind(%s) = %v", raw, err)
		}
	}
}
