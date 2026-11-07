package domain

import (
	"cmp"
	"context"
	"slices"
	"time"
)

// WatchedSource is how a subscription's source last answered, as alerts see
// it: Failure is empty while the source gives figures, else the outcome
// that replaced them (KEY_MISSING, BROKEN…), with the source's hint.
type WatchedSource struct {
	Subscription string
	Failure      string
	Hint         string
}

// SourceAlert announces a source that started failing, or that fails for
// another reason than before.
type SourceAlert struct {
	Subscription string
	Failure      string
	Hint         string
}

// FailingSource is the memory that a source was announced as failing.
type FailingSource struct {
	Subscription string
	Failure      string
	Since        time.Time
}

// Failing lists the sources announced as failing, by subscription.
func (s AlertState) Failing() []FailingSource {
	failing := append([]FailingSource(nil), s.failing...)
	slices.SortFunc(failing, func(a, b FailingSource) int { return cmp.Compare(a.Subscription, b.Subscription) })
	return failing
}

// EvaluateSources announces each source that starts failing, or fails for
// another reason, once; a source that recovers, or is no longer watched,
// is forgotten, so failing again is announced again.
func (s AlertState) EvaluateSources(sources []WatchedSource, now time.Time) (AlertState, []SourceAlert) {
	next := AlertState{raised: s.raised}
	var alerts []SourceAlert
	for _, source := range sources {
		if source.Failure == "" {
			continue
		}
		if known, found := s.failingSource(source.Subscription); found && known.Failure == source.Failure {
			next.failing = append(next.failing, known)
			continue
		}
		next.failing = append(next.failing, FailingSource{Subscription: source.Subscription, Failure: source.Failure, Since: now})
		alerts = append(alerts, SourceAlert(source))
	}
	return next, alerts
}

func (s AlertState) failingSource(subscription string) (FailingSource, bool) {
	for _, failing := range s.failing {
		if failing.Subscription == subscription {
			return failing, true
		}
	}
	return FailingSource{}, false
}

// SourceNotifier shows a failing source to the owner.
type SourceNotifier interface {
	NotifySourceFailure(ctx context.Context, alert SourceAlert) error
}
