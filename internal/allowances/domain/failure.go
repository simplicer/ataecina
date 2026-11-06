package domain

import (
	"strings"
	"time"
	"unicode"
)

const maxHintLength = 200

// SourceFailure explains why a subscription could not be read. Adapters
// classify provider errors into one of these; the hint is a short, safe
// sentence for the owner (never a key, a body or file contents).
type SourceFailure struct {
	outcome Outcome
	class   ErrorClass
	hint    string
	retryAt time.Time
	cause   error
}

// Failure classifies a provider or file failure (SPECS §4.3).
func Failure(class ErrorClass, hint string, cause error) *SourceFailure {
	outcome := OutcomeUnavailable
	switch class {
	case ErrorKeyRejected:
		outcome = OutcomeKeyMissing
	case ErrorContractChanged:
		outcome = OutcomeBroken
	}
	return &SourceFailure{outcome: outcome, class: class, hint: cleanHint(hint), cause: cause}
}

// RateLimited asks the collector not to try the source again before retryAt.
func RateLimited(retryAt time.Time, hint string) *SourceFailure {
	failure := Failure(ErrorRateLimited, hint, nil)
	failure.retryAt = retryAt.UTC()
	return failure
}

// Unsupported reports a source this build cannot read.
func Unsupported(hint string) *SourceFailure {
	return &SourceFailure{outcome: OutcomeUnsupported, hint: cleanHint(hint)}
}

// KeyMissing reports a subscription whose key has not been set.
func KeyMissing(hint string) *SourceFailure {
	return &SourceFailure{outcome: OutcomeKeyMissing, hint: cleanHint(hint)}
}

// KeysLocked reports that no unlocker could open the master key.
func KeysLocked(hint string) *SourceFailure {
	return &SourceFailure{outcome: OutcomeLocked, hint: cleanHint(hint)}
}

func (f *SourceFailure) Error() string {
	label := string(f.outcome)
	if f.class != "" {
		label = string(f.class)
	}
	if f.hint == "" {
		return label
	}
	return label + ": " + f.hint
}

// Unwrap exposes the underlying error for logging by class only.
func (f *SourceFailure) Unwrap() error { return f.cause }

// Class is the error class, empty for unsupported, missing or locked keys.
func (f *SourceFailure) Class() ErrorClass { return f.class }

// Outcome is what the subscription shows when nothing better is known.
func (f *SourceFailure) Outcome() Outcome { return f.outcome }

// Hint is the owner-facing explanation.
func (f *SourceFailure) Hint() string { return f.hint }

// RetryAt is when the source may be asked again, when the provider said so.
func (f *SourceFailure) RetryAt() (time.Time, bool) { return f.retryAt, !f.retryAt.IsZero() }

func cleanHint(hint string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(hint))
	runes := []rune(cleaned)
	if len(runes) > maxHintLength {
		cleaned = string(runes[:maxHintLength-1]) + "…"
	}
	return cleaned
}
