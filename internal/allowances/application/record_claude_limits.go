package application

import (
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// RecordClaudeLimits keeps the rate limits Claude Code hands to its status
// line as a signal for the collector (SPECS §3.1). It must be quick and
// must never make the status line fail; the caller ignores its errors.
type RecordClaudeLimits struct {
	signals domain.LimitSignals
	clock   domain.Clock
}

// NewRecordClaudeLimits prepares the hook.
func NewRecordClaudeLimits(signals domain.LimitSignals, clock domain.Clock) RecordClaudeLimits {
	return RecordClaudeLimits{signals: signals, clock: clock}
}

// Record merges the reported meters with the last signal and keeps the
// result only when something is newer. It returns what the subscription
// shows now, or domain.ErrNoSignal when nothing is known yet.
func (r RecordClaudeLimits) Record(subscription domain.Subscription, reported []domain.Meter) (domain.Reading, error) {
	previous, err := r.signals.LatestSignal(subscription.ID())
	hasPrevious := err == nil
	if len(reported) == 0 {
		if !hasPrevious {
			return domain.Reading{}, domain.ErrNoSignal
		}
		return previous, nil
	}
	var known []domain.Meter
	if hasPrevious {
		known = previous.Meters()
	}
	now := r.clock.Now()
	merged, changed := domain.CarryForward(known, reported, now)
	if !changed {
		return previous, nil
	}
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(),
		Provider:     subscription.Provider(),
		Source:       subscription.Source(),
		Fidelity:     domain.FidelityReported,
		Outcome:      domain.OutcomeOK,
		ObservedAt:   now,
		Meters:       merged,
	})
	if err != nil {
		return domain.Reading{}, fmt.Errorf("record Claude limits: %w", err)
	}
	if err := r.signals.KeepSignal(reading); err != nil {
		return reading, fmt.Errorf("record Claude limits: %w", err)
	}
	return reading, nil
}
