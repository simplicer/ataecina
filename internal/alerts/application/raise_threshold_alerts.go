// Package application holds the alerts use cases, one per file.
package application

import (
	"context"
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

// RaiseThresholdAlerts compares the collected meters with their
// thresholds, notifies each newly crossed threshold once per window and
// keeps the state (SPECS §5, §7).
type RaiseThresholdAlerts struct {
	states   domain.AlertStates
	notifier domain.Notifier
	clock    domain.Clock
}

// NewRaiseThresholdAlerts prepares the alerts of one collection.
func NewRaiseThresholdAlerts(states domain.AlertStates, notifier domain.Notifier, clock domain.Clock) RaiseThresholdAlerts {
	return RaiseThresholdAlerts{states: states, notifier: notifier, clock: clock}
}

// Raise evaluates the meters and returns the alerts announced. A failed
// notification does not bring the alert back next time: the state is kept
// and the failure returned for the log.
func (r RaiseThresholdAlerts) Raise(ctx context.Context, meters []domain.WatchedMeter) ([]domain.Alert, error) {
	state, err := r.states.Load()
	if err != nil {
		state = domain.NewAlertState(nil)
	}
	next, alerts := state.Evaluate(meters, r.clock.Now())
	if err := r.states.Save(next); err != nil {
		return nil, fmt.Errorf("keep the alert state: %w", err)
	}
	var failures []error
	for _, alert := range alerts {
		if err := r.notifier.Notify(ctx, alert); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return alerts, fmt.Errorf("notify: %w", errors.Join(failures...))
	}
	return alerts, nil
}
