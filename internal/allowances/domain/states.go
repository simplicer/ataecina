package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrUnknownValue reports an enumeration value Ataecina does not know.
var ErrUnknownValue = errors.New("unknown value")

// Fidelity says how far a figure can be trusted (ADR 0011).
type Fidelity string

// Fidelities.
const (
	FidelityReported Fidelity = "REPORTED"
	FidelityDerived  Fidelity = "DERIVED"
	FidelityObserved Fidelity = "OBSERVED"
	FidelityDeclared Fidelity = "DECLARED"
)

// ParseFidelity restores a stored fidelity.
func ParseFidelity(raw string) (Fidelity, error) {
	switch f := Fidelity(raw); f {
	case FidelityReported, FidelityDerived, FidelityObserved, FidelityDeclared:
		return f, nil
	}
	return "", fmt.Errorf("%w: fidelity %q", ErrUnknownValue, raw)
}

// Outcome is the result of the latest attempt to read a subscription.
type Outcome string

// Outcomes (SPECS §4.1).
const (
	OutcomeOK          Outcome = "OK"
	OutcomePartial     Outcome = "PARTIAL"
	OutcomeBroken      Outcome = "BROKEN"
	OutcomeUnavailable Outcome = "UNAVAILABLE"
	OutcomeKeyMissing  Outcome = "KEY_MISSING"
	OutcomeLocked      Outcome = "LOCKED"
	OutcomeUnsupported Outcome = "UNSUPPORTED"
)

// ParseOutcome restores a stored outcome.
func ParseOutcome(raw string) (Outcome, error) {
	switch o := Outcome(raw); o {
	case OutcomeOK, OutcomePartial, OutcomeBroken, OutcomeUnavailable,
		OutcomeKeyMissing, OutcomeLocked, OutcomeUnsupported:
		return o, nil
	}
	return "", fmt.Errorf("%w: outcome %q", ErrUnknownValue, raw)
}

// HasFigures reports whether a reading with this outcome carries figures.
func (o Outcome) HasFigures() bool { return o == OutcomeOK || o == OutcomePartial }

// Freshness tells how old a reading is relative to the collection interval.
type Freshness string

// Freshness values (SPECS §4.2).
const (
	FreshnessFresh   Freshness = "FRESH"
	FreshnessStale   Freshness = "STALE"
	FreshnessUnknown Freshness = "UNKNOWN"
)

// FreshnessOf is FRESH while the age is under twice the interval, STALE
// afterwards and UNKNOWN when nothing was ever observed.
func FreshnessOf(observedAt, now time.Time, interval time.Duration) Freshness {
	if observedAt.IsZero() {
		return FreshnessUnknown
	}
	if now.Sub(observedAt) < 2*interval {
		return FreshnessFresh
	}
	return FreshnessStale
}

// ErrorClass classifies why a source could not be read (SPECS §4.3).
type ErrorClass string

// Error classes.
const (
	ErrorKeyRejected     ErrorClass = "KEY_REJECTED"
	ErrorRateLimited     ErrorClass = "RATE_LIMITED"
	ErrorUnavailable     ErrorClass = "UNAVAILABLE"
	ErrorContractChanged ErrorClass = "CONTRACT_CHANGED"
	ErrorUnreadable      ErrorClass = "UNREADABLE"
)

// ParseErrorClass restores a stored error class; empty means none.
func ParseErrorClass(raw string) (ErrorClass, error) {
	switch c := ErrorClass(raw); c {
	case "", ErrorKeyRejected, ErrorRateLimited, ErrorUnavailable, ErrorContractChanged, ErrorUnreadable:
		return c, nil
	}
	return "", fmt.Errorf("%w: error class %q", ErrUnknownValue, raw)
}

// IsTransient reports failures after which the last good reading is kept.
func (c ErrorClass) IsTransient() bool {
	return c == ErrorRateLimited || c == ErrorUnavailable || c == ErrorUnreadable
}
