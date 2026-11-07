package application

import (
	"context"
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

// NotifyFailingSources tells the owner once when a subscription's source
// starts failing, or fails for another reason, and forgets it when it
// recovers (SPECS §7).
type NotifyFailingSources struct {
	states   domain.AlertStates
	notifier domain.SourceNotifier
	clock    domain.Clock
}

// NewNotifyFailingSources prepares the source alerts of one collection.
func NewNotifyFailingSources(states domain.AlertStates, notifier domain.SourceNotifier, clock domain.Clock) NotifyFailingSources {
	return NotifyFailingSources{states: states, notifier: notifier, clock: clock}
}

// Notify evaluates the sources and returns the failures announced. A failed
// notification does not bring the failure back next time: the state is
// kept and the failure returned for the log.
func (n NotifyFailingSources) Notify(ctx context.Context, sources []domain.WatchedSource) ([]domain.SourceAlert, error) {
	state, err := n.states.Load()
	if err != nil {
		state = domain.NewAlertState(nil)
	}
	next, alerts := state.EvaluateSources(sources, n.clock.Now())
	if err := n.states.Save(next); err != nil {
		return nil, fmt.Errorf("keep the alert state: %w", err)
	}
	var failures []error
	for _, alert := range alerts {
		if err := n.notifier.NotifySourceFailure(ctx, alert); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return alerts, fmt.Errorf("notify: %w", errors.Join(failures...))
	}
	return alerts, nil
}
