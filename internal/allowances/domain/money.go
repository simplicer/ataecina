package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ErrCurrencyMismatch reports arithmetic between two currencies.
var ErrCurrencyMismatch = errors.New("currencies differ")

const (
	microsPerUnit = 1_000_000
	// maxAmountUnits keeps every amount far inside int64 micros.
	maxAmountUnits = 1_000_000_000_000
	amountDecimals = 6
)

var (
	decimalPattern  = regexp.MustCompile(`^[+-]?([0-9]+)(\.([0-9]+))?$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

// Amount is an exact decimal with six fractional digits, enough for
// providers that bill in micro-dollars.
type Amount struct {
	micros int64
}

// ParseAmount reads a decimal such as "12.30", "-3.5" or "1e-3". Digits
// beyond the sixth decimal are rounded half away from zero.
func ParseAmount(raw string) (Amount, error) {
	text := strings.TrimSpace(raw)
	if strings.ContainsAny(text, "eE") {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return Amount{}, fmt.Errorf("%w: amount %q", ErrInvalidFigure, raw)
		}
		return AmountFromFloat(value)
	}
	parts := decimalPattern.FindStringSubmatch(text)
	if parts == nil {
		return Amount{}, fmt.Errorf("%w: amount %q", ErrInvalidFigure, raw)
	}
	whole, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || whole > maxAmountUnits {
		return Amount{}, fmt.Errorf("%w: amount %q is too large", ErrInvalidFigure, raw)
	}
	fraction := parts[3] + strings.Repeat("0", amountDecimals+1)
	micros, _ := strconv.ParseInt(fraction[:amountDecimals], 10, 64)
	if fraction[amountDecimals] >= '5' {
		micros++
	}
	micros += whole * microsPerUnit
	if strings.HasPrefix(text, "-") {
		micros = -micros
	}
	return Amount{micros}, nil
}

// AmountFromFloat converts a JSON number, rounding to six decimals.
func AmountFromFloat(value float64) (Amount, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > maxAmountUnits {
		return Amount{}, fmt.Errorf("%w: amount %v", ErrInvalidFigure, value)
	}
	return Amount{int64(math.Round(value * microsPerUnit))}, nil
}

// Add returns a + other.
func (a Amount) Add(other Amount) Amount { return Amount{a.micros + other.micros} }

// Sub returns a − other.
func (a Amount) Sub(other Amount) Amount { return Amount{a.micros - other.micros} }

// IsNegative reports whether the amount is below zero.
func (a Amount) IsNegative() bool { return a.micros < 0 }

// Float64 is the amount as a JSON number.
func (a Amount) Float64() float64 { return float64(a.micros) / microsPerUnit }

// String renders the amount without trailing zeros ("12.3", "0").
func (a Amount) String() string {
	sign := ""
	micros := a.micros
	if micros < 0 {
		sign, micros = "-", -micros
	}
	whole := micros / microsPerUnit
	fraction := strings.TrimRight(fmt.Sprintf("%06d", micros%microsPerUnit), "0")
	if fraction == "" {
		return sign + strconv.FormatInt(whole, 10)
	}
	return fmt.Sprintf("%s%d.%s", sign, whole, fraction)
}

// Currency is an ISO 4217 code (USD, EUR).
type Currency string

// NewCurrency validates an ISO 4217 code.
func NewCurrency(raw string) (Currency, error) {
	if !currencyPattern.MatchString(raw) {
		return "", fmt.Errorf("%w: currency %q is not ISO 4217", ErrInvalidFigure, raw)
	}
	return Currency(raw), nil
}

func (c Currency) String() string { return string(c) }

// Money is an amount in one currency.
type Money struct {
	amount   Amount
	currency Currency
}

// NewMoney joins an amount and its currency.
func NewMoney(amount Amount, currency Currency) Money {
	return Money{amount: amount, currency: currency}
}

// Amount is the amount.
func (m Money) Amount() Amount { return m.amount }

// Currency is the ISO 4217 currency.
func (m Money) Currency() Currency { return m.currency }

// Add returns m + other in the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{m.amount.Add(other.amount), m.currency}, nil
}

// Sub returns m − other in the same currency.
func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{m.amount.Sub(other.amount), m.currency}, nil
}

// Balance is prepaid money left and spent; either may be unknown.
type Balance struct {
	remaining    Money
	hasRemaining bool
	spent        Money
	hasSpent     bool
}

// NewBalance needs at least one figure; both must share a currency.
func NewBalance(remaining, spent *Money) (Balance, error) {
	if remaining == nil && spent == nil {
		return Balance{}, fmt.Errorf("%w: a balance needs a remaining or a spent amount", ErrInvalidFigure)
	}
	if remaining != nil && spent != nil && remaining.currency != spent.currency {
		return Balance{}, fmt.Errorf("%w: remaining in %s, spent in %s", ErrCurrencyMismatch, remaining.currency, spent.currency)
	}
	var balance Balance
	if remaining != nil {
		balance.remaining, balance.hasRemaining = *remaining, true
	}
	if spent != nil {
		balance.spent, balance.hasSpent = *spent, true
	}
	return balance, nil
}

// Remaining is the money left, when known.
func (b Balance) Remaining() (Money, bool) { return b.remaining, b.hasRemaining }

// Spent is the money spent, when known.
func (b Balance) Spent() (Money, bool) { return b.spent, b.hasSpent }

// Credits are provider units that are not money (Codex credits).
type Credits struct {
	available Amount
	unlimited bool
}

// NewCredits records the credits available and whether they are unlimited.
func NewCredits(available Amount, unlimited bool) Credits {
	return Credits{available: available, unlimited: unlimited}
}

// Available is the credit balance.
func (c Credits) Available() Amount { return c.available }

// Unlimited reports an unlimited credit plan.
func (c Credits) Unlimited() bool { return c.unlimited }

// BankedResets counts window resets the owner may trigger early (Codex);
// Ataecina shows them as information and never spends them.
type BankedResets struct {
	available int
}

// NewBankedResets validates the banked resets count.
func NewBankedResets(available int) (BankedResets, error) {
	if available < 0 {
		return BankedResets{}, fmt.Errorf("%w: %d banked resets", ErrInvalidFigure, available)
	}
	return BankedResets{available: available}, nil
}

// Available is how many resets are banked.
func (b BankedResets) Available() int { return b.available }
