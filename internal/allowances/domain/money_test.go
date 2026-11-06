package domain_test

import (
	"errors"
	"math"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func mustAmount(t *testing.T, raw string) domain.Amount {
	t.Helper()
	amount, err := domain.ParseAmount(raw)
	if err != nil {
		t.Fatalf("ParseAmount(%q) = %v", raw, err)
	}
	return amount
}

func TestAmountsKeepDecimalsExactly(t *testing.T) {
	cases := map[string]string{
		"12.30":      "12.3",
		"0":          "0",
		"-3.5":       "-3.5",
		"0.000001":   "0.000001",
		"0.0000014":  "0.000001",
		"0.0000015":  "0.000002",
		"+7":         "7",
		"1e-3":       "0.001",
		"2.5E2":      "250",
		"100.000000": "100",
		"-0.0000004": "0",
	}
	for raw, want := range cases {
		if got := mustAmount(t, raw).String(); got != want {
			t.Fatalf("ParseAmount(%q).String() = %q, want %q", raw, got, want)
		}
	}
	if got := mustAmount(t, "8.42").Float64(); got != 8.42 {
		t.Fatalf("Float64() = %v", got)
	}
}

func TestAmountsRejectGarbage(t *testing.T) {
	for _, raw := range []string{"", "abc", "1.2.3", "--1", "1e400", "99999999999999", ".", "1,5", "NaN"} {
		if _, err := domain.ParseAmount(raw); !errors.Is(err, domain.ErrInvalidFigure) {
			t.Fatalf("ParseAmount(%q) = %v, want ErrInvalidFigure", raw, err)
		}
	}
}

func TestAmountFromFloat(t *testing.T) {
	a, err := domain.AmountFromFloat(11.58)
	if err != nil || a.String() != "11.58" {
		t.Fatalf("AmountFromFloat = %v, %v", a, err)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(-1), 1e15} {
		if _, err := domain.AmountFromFloat(bad); !errors.Is(err, domain.ErrInvalidFigure) {
			t.Fatalf("AmountFromFloat(%v) = %v", bad, err)
		}
	}
}

func TestAmountArithmetic(t *testing.T) {
	sum := mustAmount(t, "20").Add(mustAmount(t, "5.25"))
	if sum.String() != "25.25" {
		t.Fatalf("Add = %s", sum)
	}
	diff := mustAmount(t, "3").Sub(mustAmount(t, "4.5"))
	if diff.String() != "-1.5" || !diff.IsNegative() {
		t.Fatalf("Sub = %s", diff)
	}
	if mustAmount(t, "0").IsNegative() {
		t.Fatal("zero is not negative")
	}
}

func TestCurrencyIsISO4217(t *testing.T) {
	usd, err := domain.NewCurrency("USD")
	if err != nil || usd.String() != "USD" {
		t.Fatalf("NewCurrency(USD) = %v, %v", usd, err)
	}
	for _, bad := range []string{"usd", "US", "EURO", ""} {
		if _, err := domain.NewCurrency(bad); !errors.Is(err, domain.ErrInvalidFigure) {
			t.Fatalf("NewCurrency(%q) = %v", bad, err)
		}
	}
}

func TestMoneyArithmeticNeedsTheSameCurrency(t *testing.T) {
	usd, _ := domain.NewCurrency("USD")
	eur, _ := domain.NewCurrency("EUR")
	twenty := domain.NewMoney(mustAmount(t, "20"), usd)
	spent := domain.NewMoney(mustAmount(t, "4.10"), usd)
	left, err := twenty.Sub(spent)
	if err != nil || left.Amount().String() != "15.9" || left.Currency() != usd {
		t.Fatalf("Sub = %v, %v", left, err)
	}
	more, err := twenty.Add(spent)
	if err != nil || more.Amount().String() != "24.1" {
		t.Fatalf("Add = %v, %v", more, err)
	}
	if _, err := twenty.Sub(domain.NewMoney(mustAmount(t, "1"), eur)); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("Sub across currencies = %v", err)
	}
	if _, err := twenty.Add(domain.NewMoney(mustAmount(t, "1"), eur)); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("Add across currencies = %v", err)
	}
}

func TestBalanceNeedsAFigureAndOneCurrency(t *testing.T) {
	usd, _ := domain.NewCurrency("USD")
	eur, _ := domain.NewCurrency("EUR")
	remaining := domain.NewMoney(mustAmount(t, "8.42"), usd)
	spent := domain.NewMoney(mustAmount(t, "11.58"), usd)

	balance, err := domain.NewBalance(&remaining, &spent)
	if err != nil {
		t.Fatalf("NewBalance = %v", err)
	}
	if got, ok := balance.Remaining(); !ok || got != remaining {
		t.Fatalf("Remaining() = %v, %v", got, ok)
	}
	if got, ok := balance.Spent(); !ok || got != spent {
		t.Fatalf("Spent() = %v, %v", got, ok)
	}

	onlySpent, err := domain.NewBalance(nil, &spent)
	if err != nil {
		t.Fatalf("NewBalance(nil, spent) = %v", err)
	}
	if _, ok := onlySpent.Remaining(); ok {
		t.Fatal("Remaining() must be absent")
	}
	if _, err := domain.NewBalance(nil, nil); !errors.Is(err, domain.ErrInvalidFigure) {
		t.Fatalf("NewBalance(nil, nil) = %v", err)
	}
	euros := domain.NewMoney(mustAmount(t, "1"), eur)
	if _, err := domain.NewBalance(&remaining, &euros); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("NewBalance across currencies = %v", err)
	}
}

func TestCreditsAndBankedResets(t *testing.T) {
	credits := domain.NewCredits(mustAmount(t, "12.5"), false)
	if credits.Available().String() != "12.5" || credits.Unlimited() {
		t.Fatalf("credits = %v", credits)
	}
	banked, err := domain.NewBankedResets(2)
	if err != nil || banked.Available() != 2 {
		t.Fatalf("NewBankedResets = %v, %v", banked, err)
	}
	if _, err := domain.NewBankedResets(-1); !errors.Is(err, domain.ErrInvalidFigure) {
		t.Fatalf("NewBankedResets(-1) = %v", err)
	}
}
