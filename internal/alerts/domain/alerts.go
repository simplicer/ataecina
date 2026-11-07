// Package domain holds the alerts model: thresholds, the meters being
// watched, the alerts raised once per threshold per window, the sources
// announced once when they start failing, and when that memory is
// forgotten (SPECS §7). It knows nothing of how meters are read or how a
// notification is shown.
package domain

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// ErrInvalidThreshold reports a threshold outside 1–100.
var ErrInvalidThreshold = errors.New("a threshold is a percentage between 1 and 100")

// Threshold is a percentage of a meter that raises an alert when crossed.
type Threshold int

// NewThreshold validates a threshold.
func NewThreshold(percent int) (Threshold, error) {
	if percent < 1 || percent > 100 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidThreshold, percent)
	}
	return Threshold(percent), nil
}

// WindowInstance names one occurrence of one window of one subscription:
// a new reset instant is a new window, with its own alerts.
type WindowInstance struct {
	Subscription string
	Bucket       string
	Window       string
	ResetsAt     time.Time
}

// WatchedMeter is a meter as alerts see it.
type WatchedMeter struct {
	Instance    WindowInstance
	UsedPercent float64
	Thresholds  []Threshold
}

// Alert is one threshold crossed by one window.
type Alert struct {
	Instance    WindowInstance
	Threshold   Threshold
	UsedPercent float64
}

// RaisedAlert is the memory that an alert was raised.
type RaisedAlert struct {
	Instance  WindowInstance
	Threshold Threshold
	RaisedAt  time.Time
}

// AlertState is every alert already raised and every source already
// announced as failing.
type AlertState struct {
	raised  []RaisedAlert
	failing []FailingSource
}

// NewAlertState restores the alerts already raised and the sources already
// announced as failing.
func NewAlertState(raised []RaisedAlert, failing ...FailingSource) AlertState {
	return AlertState{raised: append([]RaisedAlert(nil), raised...), failing: append([]FailingSource(nil), failing...)}
}

// Raised lists the alerts already raised, in a stable order.
func (s AlertState) Raised() []RaisedAlert {
	raised := append([]RaisedAlert(nil), s.raised...)
	slices.SortFunc(raised, func(a, b RaisedAlert) int {
		return cmp.Or(
			cmp.Compare(a.Instance.Subscription, b.Instance.Subscription),
			cmp.Compare(a.Instance.Bucket, b.Instance.Bucket),
			cmp.Compare(a.Instance.Window, b.Instance.Window),
			cmp.Compare(a.Threshold, b.Threshold),
		)
	})
	return raised
}

// sameResetTolerance absorbs reset instants a provider computes anew at
// every answer ("resets in N seconds").
const sameResetTolerance = time.Minute

// Same reports whether two instances are the same occurrence of a window.
func (w WindowInstance) Same(other WindowInstance) bool {
	gap := w.ResetsAt.Sub(other.ResetsAt)
	return w.Subscription == other.Subscription && w.Bucket == other.Bucket && w.Window == other.Window &&
		gap <= sameResetTolerance && gap >= -sameResetTolerance
}

// Evaluate compares the meters with their thresholds. Each crossed
// threshold is remembered once per window instance; when several are
// crossed at once only the highest is announced. The memory of windows no
// longer watched (reborn, or gone) is forgotten.
func (s AlertState) Evaluate(meters []WatchedMeter, now time.Time) (AlertState, []Alert) {
	next := AlertState{failing: s.failing}
	for _, raised := range s.raised {
		for _, meter := range meters {
			if raised.Instance.Same(meter.Instance) {
				raised.Instance = meter.Instance
				next.raised = append(next.raised, raised)
				break
			}
		}
	}
	var alerts []Alert
	for _, meter := range meters {
		var highest *Alert
		for _, threshold := range meter.Thresholds {
			if meter.UsedPercent < float64(threshold) || next.has(meter.Instance, threshold) {
				continue
			}
			next.raised = append(next.raised, RaisedAlert{Instance: meter.Instance, Threshold: threshold, RaisedAt: now})
			if highest == nil || threshold > highest.Threshold {
				highest = &Alert{Instance: meter.Instance, Threshold: threshold, UsedPercent: meter.UsedPercent}
			}
		}
		if highest != nil {
			alerts = append(alerts, *highest)
		}
	}
	return next, alerts
}

func (s AlertState) has(instance WindowInstance, threshold Threshold) bool {
	for _, raised := range s.raised {
		if raised.Instance.Same(instance) && raised.Threshold == threshold {
			return true
		}
	}
	return false
}

// AlertStates keeps the alert state; Load gives an empty state at first.
type AlertStates interface {
	Load() (AlertState, error)
	Save(state AlertState) error
}

// Notifier shows an alert to the owner.
type Notifier interface {
	Notify(ctx context.Context, alert Alert) error
}

// Clock tells the time; the domain itself never reads it.
type Clock interface {
	Now() time.Time
}
