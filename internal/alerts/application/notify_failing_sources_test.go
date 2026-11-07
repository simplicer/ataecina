package application_test

import (
	"context"
	"errors"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/alerts/application"
	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

type sourceNotifier struct {
	shown []domain.SourceAlert
	err   error
}

func (n *sourceNotifier) NotifySourceFailure(_ context.Context, alert domain.SourceAlert) error {
	n.shown = append(n.shown, alert)
	return n.err
}

var failing = []domain.WatchedSource{
	{Subscription: "workers-ai", Failure: "KEY_MISSING", Hint: "the token needs Account Analytics: Read"},
	{Subscription: "zai"},
}

func TestAFailingSourceIsNotifiedOnce(t *testing.T) {
	s, n := &shelf{}, &sourceNotifier{}
	notify := application.NewNotifyFailingSources(s, n, clock{})
	if alerts, err := notify.Notify(context.Background(), failing); err != nil || len(alerts) != 1 || len(n.shown) != 1 || n.shown[0].Subscription != "workers-ai" {
		t.Fatalf("first = %v, %v, %+v", alerts, err, n.shown)
	}
	if alerts, err := notify.Notify(context.Background(), failing); err != nil || len(alerts) != 0 || len(n.shown) != 1 {
		t.Fatalf("second = %v, %v", alerts, err)
	}
}

func TestSourceNotificationFailuresAreReportedWithoutRepeating(t *testing.T) {
	s := &shelf{}
	broken := &sourceNotifier{err: errors.New("no notification daemon")}
	if alerts, err := application.NewNotifyFailingSources(s, broken, clock{}).Notify(context.Background(), failing); err == nil || len(alerts) != 1 {
		t.Fatalf("a failed notification is reported with its alert: %v, %v", alerts, err)
	}
	if len(s.state.Failing()) != 1 {
		t.Fatal("the failure is remembered even when it could not be shown")
	}
	if _, err := application.NewNotifyFailingSources(&shelf{saveErr: errors.New("read-only")}, &sourceNotifier{}, clock{}).Notify(context.Background(), failing); err == nil {
		t.Fatal("a state that cannot be kept is reported")
	}
	n := &sourceNotifier{}
	if _, err := application.NewNotifyFailingSources(&shelf{loadErr: errors.New("corrupt")}, n, clock{}).Notify(context.Background(), failing); err != nil || len(n.shown) != 1 {
		t.Fatalf("a corrupt state starts again: %v", err)
	}
}
