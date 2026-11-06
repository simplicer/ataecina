package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoDeclaration reports a balance-only subscription the owner has not
// declared yet.
var ErrNoDeclaration = errors.New("no balance declared yet")

// DeclarationKind says what the owner declared.
type DeclarationKind string

// Declaration kinds.
const (
	// DeclaredTopUp adds money to the balance.
	DeclaredTopUp DeclarationKind = "TOP_UP"
	// DeclaredBalance states the whole current balance, replacing what came before.
	DeclaredBalance DeclarationKind = "BALANCE"
)

// ParseDeclarationKind restores a stored kind.
func ParseDeclarationKind(raw string) (DeclarationKind, error) {
	switch kind := DeclarationKind(raw); kind {
	case DeclaredTopUp, DeclaredBalance:
		return kind, nil
	}
	return "", fmt.Errorf("%w: declaration kind %q", ErrUnknownValue, raw)
}

// Declaration is a top-up or a current balance typed by the owner, with
// the money already spent to date at that moment (the spend baseline), so
// later spending can be derived from local records.
type Declaration struct {
	kind          DeclarationKind
	amount        Money
	declaredAt    time.Time
	spentBaseline Money
}

// NewDeclaration validates a declaration.
func NewDeclaration(kind DeclarationKind, amount Money, declaredAt time.Time, spentBaseline Money) (Declaration, error) {
	if _, err := ParseDeclarationKind(string(kind)); err != nil {
		return Declaration{}, err
	}
	if amount.amount.IsNegative() || (kind == DeclaredTopUp && amount.amount.micros == 0) {
		return Declaration{}, fmt.Errorf("%w: a top-up must be positive and a balance not negative", ErrInvalidFigure)
	}
	if amount.currency != spentBaseline.currency {
		return Declaration{}, fmt.Errorf("%w: declared in %s, spending recorded in %s", ErrCurrencyMismatch, amount.currency, spentBaseline.currency)
	}
	if declaredAt.IsZero() {
		return Declaration{}, fmt.Errorf("%w: a declaration needs its time", ErrInvalidFigure)
	}
	return Declaration{kind: kind, amount: amount, declaredAt: declaredAt.UTC(), spentBaseline: spentBaseline}, nil
}

// Kind is a top-up or a balance.
func (d Declaration) Kind() DeclarationKind { return d.kind }

// Amount is the money declared.
func (d Declaration) Amount() Money { return d.amount }

// DeclaredAt is when it was declared.
func (d Declaration) DeclaredAt() time.Time { return d.declaredAt }

// SpentBaseline is the money spent to date when it was declared.
func (d Declaration) SpentBaseline() Money { return d.spentBaseline }

// DeriveBalance works out a prepaid balance from the owner's declarations
// and the money spent to date: the latest declared balance (or zero) plus
// the top-ups after it, minus what was spent since that reference. Spent
// never goes below zero (local records may have been cleared); remaining
// may, when the owner forgot to declare a top-up.
func DeriveBalance(declarations []Declaration, spentToDate Money) (Balance, error) {
	if len(declarations) == 0 {
		return Balance{}, ErrNoDeclaration
	}
	reference := 0
	for i, declaration := range declarations {
		if declaration.kind == DeclaredBalance {
			reference = i
		}
	}
	start := declarations[reference]
	remaining := Money{currency: start.amount.currency}
	if start.kind == DeclaredBalance {
		remaining = start.amount
	}
	for _, declaration := range declarations[reference:] {
		if declaration.kind != DeclaredTopUp {
			continue
		}
		var err error
		if remaining, err = remaining.Add(declaration.amount); err != nil {
			return Balance{}, err
		}
	}
	spent, err := spentToDate.Sub(start.spentBaseline)
	if err != nil {
		return Balance{}, err
	}
	if spent.amount.IsNegative() {
		spent.amount = Amount{}
	}
	if remaining, err = remaining.Sub(spent); err != nil {
		return Balance{}, err
	}
	return NewBalance(&remaining, &spent)
}

// BalanceDeclarations keeps the owner's declarations per subscription.
type BalanceDeclarations interface {
	DeclarationsOf(subscription SubscriptionID) ([]Declaration, error)
	KeepDeclaration(subscription SubscriptionID, declaration Declaration) error
}

// RecordedSpend tells the money spent to date according to local records
// (OpenCode's own statistics for OpenCode Zen).
type RecordedSpend interface {
	SpentToDate(ctx context.Context) (Money, error)
}
