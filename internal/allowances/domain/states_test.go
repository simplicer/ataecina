package domain_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func TestEnumerationsRoundTrip(t *testing.T) {
	for _, raw := range []string{"REPORTED", "DERIVED", "OBSERVED", "DECLARED"} {
		if f, err := domain.ParseFidelity(raw); err != nil || string(f) != raw {
			t.Fatalf("ParseFidelity(%s) = %v, %v", raw, f, err)
		}
	}
	for _, raw := range []string{"OK", "PARTIAL", "BROKEN", "UNAVAILABLE", "KEY_MISSING", "LOCKED", "UNSUPPORTED"} {
		if o, err := domain.ParseOutcome(raw); err != nil || string(o) != raw {
			t.Fatalf("ParseOutcome(%s) = %v, %v", raw, o, err)
		}
	}
	for _, raw := range []string{"", "KEY_REJECTED", "RATE_LIMITED", "UNAVAILABLE", "CONTRACT_CHANGED", "UNREADABLE"} {
		if c, err := domain.ParseErrorClass(raw); err != nil || string(c) != raw {
			t.Fatalf("ParseErrorClass(%s) = %v, %v", raw, c, err)
		}
	}
	if _, err := domain.ParseFidelity("GUESSED"); !errors.Is(err, domain.ErrUnknownValue) {
		t.Fatalf("ParseFidelity(GUESSED) = %v", err)
	}
	if _, err := domain.ParseOutcome("FINE"); !errors.Is(err, domain.ErrUnknownValue) {
		t.Fatalf("ParseOutcome(FINE) = %v", err)
	}
	if _, err := domain.ParseErrorClass("OOPS"); !errors.Is(err, domain.ErrUnknownValue) {
		t.Fatalf("ParseErrorClass(OOPS) = %v", err)
	}
}

func TestOnlyOKAndPartialCarryFigures(t *testing.T) {
	with := map[domain.Outcome]bool{
		domain.OutcomeOK: true, domain.OutcomePartial: true,
		domain.OutcomeBroken: false, domain.OutcomeUnavailable: false,
		domain.OutcomeKeyMissing: false, domain.OutcomeLocked: false, domain.OutcomeUnsupported: false,
	}
	for outcome, want := range with {
		if outcome.HasFigures() != want {
			t.Fatalf("%s.HasFigures() = %v", outcome, !want)
		}
	}
}

func TestFreshnessIsTwiceTheInterval(t *testing.T) {
	now := time.Date(2026, 11, 6, 20, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	if got := domain.FreshnessOf(now.Add(-9*time.Minute), now, interval); got != domain.FreshnessFresh {
		t.Fatalf("9 minutes old = %s", got)
	}
	if got := domain.FreshnessOf(now.Add(-10*time.Minute), now, interval); got != domain.FreshnessStale {
		t.Fatalf("10 minutes old = %s", got)
	}
	if got := domain.FreshnessOf(time.Time{}, now, interval); got != domain.FreshnessUnknown {
		t.Fatalf("never observed = %s", got)
	}
}

func TestTransientErrorClasses(t *testing.T) {
	for class, want := range map[domain.ErrorClass]bool{
		domain.ErrorRateLimited: true, domain.ErrorUnavailable: true, domain.ErrorUnreadable: true,
		domain.ErrorKeyRejected: false, domain.ErrorContractChanged: false,
	} {
		if class.IsTransient() != want {
			t.Fatalf("%s.IsTransient() = %v", class, !want)
		}
	}
}
