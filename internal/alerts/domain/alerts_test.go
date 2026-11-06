package domain_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

var defaults = []domain.Threshold{90, 95}

func meter(used float64, resetsAt time.Time) domain.WatchedMeter {
	return domain.WatchedMeter{
		Instance:    domain.WindowInstance{Subscription: "claude", Window: "FIVE_HOURS", ResetsAt: resetsAt},
		UsedPercent: used,
		Thresholds:  defaults,
	}
}

func TestEachThresholdIsAnnouncedOncePerWindow(t *testing.T) {
	reset := now.Add(2 * time.Hour)
	state, alerts := domain.NewAlertState(nil).Evaluate([]domain.WatchedMeter{meter(91, reset)}, now)
	if len(alerts) != 1 || alerts[0].Threshold != 90 {
		t.Fatalf("alerts = %+v", alerts)
	}
	state, alerts = state.Evaluate([]domain.WatchedMeter{meter(93, reset)}, now)
	if len(alerts) != 0 {
		t.Fatal("90 % was already announced in this window")
	}
	state, alerts = state.Evaluate([]domain.WatchedMeter{meter(96, reset)}, now)
	if len(alerts) != 1 || alerts[0].Threshold != 95 || alerts[0].UsedPercent != 96 {
		t.Fatalf("crossing 95 = %+v", alerts)
	}
	if len(state.Raised()) != 2 {
		t.Fatalf("raised = %+v", state.Raised())
	}
}

func TestAResetComputedAnewIsTheSameWindow(t *testing.T) {
	state, _ := domain.NewAlertState(nil).Evaluate([]domain.WatchedMeter{meter(91, now.Add(time.Hour))}, now)
	state, alerts := state.Evaluate([]domain.WatchedMeter{meter(92, now.Add(time.Hour+40*time.Second))}, now)
	if len(alerts) != 0 || len(state.Raised()) != 1 {
		t.Fatalf("a reset 40 s later is the same window: %+v", alerts)
	}
	if !state.Raised()[0].Instance.ResetsAt.Equal(now.Add(time.Hour + 40*time.Second)) {
		t.Fatal("the remembered instance follows the latest reset")
	}
}

func TestOnlyTheHighestOfSeveralCrossedThresholdsIsAnnounced(t *testing.T) {
	state, alerts := domain.NewAlertState(nil).Evaluate([]domain.WatchedMeter{meter(97, now.Add(time.Hour))}, now)
	if len(alerts) != 1 || alerts[0].Threshold != 95 || len(state.Raised()) != 2 {
		t.Fatalf("alerts = %+v, raised = %+v", alerts, state.Raised())
	}
}

func TestARebornWindowStartsAgain(t *testing.T) {
	first := now.Add(time.Hour)
	state, _ := domain.NewAlertState(nil).Evaluate([]domain.WatchedMeter{meter(96, first)}, now)
	next := first.Add(5 * time.Hour)
	state, alerts := state.Evaluate([]domain.WatchedMeter{meter(92, next)}, first)
	if len(alerts) != 1 || alerts[0].Threshold != 90 {
		t.Fatalf("a new window announces again: %+v", alerts)
	}
	if raised := state.Raised(); len(raised) != 1 || !raised[0].Instance.ResetsAt.Equal(next) {
		t.Fatalf("the old window is forgotten: %+v", raised)
	}
}

func TestThresholdsAreValidatedAndStatesOrdered(t *testing.T) {
	for _, bad := range []int{0, 101, -5} {
		if _, err := domain.NewThreshold(bad); !errors.Is(err, domain.ErrInvalidThreshold) {
			t.Fatalf("NewThreshold(%d) = %v", bad, err)
		}
	}
	if threshold, err := domain.NewThreshold(80); err != nil || threshold != 80 {
		t.Fatal("NewThreshold(80)")
	}
	instance := func(subscription, bucket string) domain.WindowInstance {
		return domain.WindowInstance{Subscription: subscription, Bucket: bucket, Window: "WEEKLY"}
	}
	state := domain.NewAlertState([]domain.RaisedAlert{
		{Instance: instance("zai", ""), Threshold: 95},
		{Instance: instance("claude", "b"), Threshold: 90},
		{Instance: instance("claude", "a"), Threshold: 95},
		{Instance: instance("claude", "a"), Threshold: 90},
	})
	raised := state.Raised()
	if raised[0].Instance.Bucket != "a" || raised[0].Threshold != 90 || raised[1].Threshold != 95 || raised[2].Instance.Bucket != "b" || raised[3].Instance.Subscription != "zai" {
		t.Fatalf("order = %+v", raised)
	}
}
