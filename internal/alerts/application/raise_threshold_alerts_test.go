package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/application"
	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

type shelf struct {
	state   *domain.AlertState
	loadErr error
	saveErr error
}

func (s *shelf) Load() (domain.AlertState, error) {
	if s.loadErr != nil {
		return domain.AlertState{}, s.loadErr
	}
	if s.state == nil {
		return domain.NewAlertState(nil), nil
	}
	return *s.state, nil
}

func (s *shelf) Save(state domain.AlertState) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.state = &state
	return nil
}

type notifier struct {
	shown []domain.Alert
	err   error
}

func (n *notifier) Notify(_ context.Context, alert domain.Alert) error {
	n.shown = append(n.shown, alert)
	return n.err
}

var hot = []domain.WatchedMeter{{
	Instance:    domain.WindowInstance{Subscription: "zai", Window: "FIVE_HOURS", ResetsAt: now.Add(time.Hour)},
	UsedPercent: 92,
	Thresholds:  []domain.Threshold{90, 95},
}}

func TestAThresholdIsNotifiedOnce(t *testing.T) {
	s, n := &shelf{}, &notifier{}
	raise := application.NewRaiseThresholdAlerts(s, n, clock{})
	if alerts, err := raise.Raise(context.Background(), hot); err != nil || len(alerts) != 1 || len(n.shown) != 1 {
		t.Fatalf("first = %v, %v", alerts, err)
	}
	if alerts, err := raise.Raise(context.Background(), hot); err != nil || len(alerts) != 0 || len(n.shown) != 1 {
		t.Fatalf("second = %v, %v", alerts, err)
	}
}

func TestFailuresAreReportedWithoutRepeatingAlerts(t *testing.T) {
	broken := &notifier{err: errors.New("no notification daemon")}
	s := &shelf{}
	if _, err := application.NewRaiseThresholdAlerts(s, broken, clock{}).Raise(context.Background(), hot); err == nil {
		t.Fatal("a failed notification is reported")
	}
	if len(s.state.Raised()) != 1 {
		t.Fatal("the alert is remembered even when it could not be shown")
	}
	if _, err := application.NewRaiseThresholdAlerts(&shelf{saveErr: errors.New("read-only")}, &notifier{}, clock{}).Raise(context.Background(), hot); err == nil {
		t.Fatal("a state that cannot be kept is reported")
	}
	n := &notifier{}
	if _, err := application.NewRaiseThresholdAlerts(&shelf{loadErr: errors.New("corrupt")}, n, clock{}).Raise(context.Background(), hot); err != nil || len(n.shown) != 1 {
		t.Fatalf("a corrupt state starts again: %v", err)
	}
}
