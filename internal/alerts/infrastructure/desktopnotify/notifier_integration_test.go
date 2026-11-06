//go:build integration

package desktopnotify_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify/notifytest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func TestAlertsReachTheDesktop(t *testing.T) {
	address := dbustest.Start(t)
	server := notifytest.Start(t, address)
	notifier := desktopnotify.New(func() (*dbus.Conn, error) { return dbus.Connect(address) }, func() time.Time { return now })
	alert := domain.Alert{
		Instance:    domain.WindowInstance{Subscription: "zai", Window: "FIVE_HOURS", ResetsAt: now.Add(130 * time.Minute)},
		Threshold:   95,
		UsedPercent: 96.4,
	}
	if err := notifier.Notify(context.Background(), alert); err != nil {
		t.Fatal(err)
	}
	shown := server.Shown()
	if len(shown) != 1 || shown[0].App != "Ataecina" || shown[0].Summary != "zai · five hours at 96 %" || shown[0].Body != "Above 95 % of this window. Reborn in 2h10m0s." || shown[0].Urgency != 2 {
		t.Fatalf("shown = %+v", shown)
	}
	alert.Threshold, alert.Instance.Bucket = 90, "premium"
	_ = notifier.Notify(context.Background(), alert)
	if last := server.Shown()[1]; last.Urgency != 1 || last.Summary != "zai · premium five hours at 96 %" {
		t.Fatalf("second = %+v", last)
	}
}

func TestWithoutADesktopTheNotificationFails(t *testing.T) {
	address := dbustest.Start(t)
	notifier := desktopnotify.New(func() (*dbus.Conn, error) { return dbus.Connect(address) }, time.Now)
	if err := notifier.Notify(context.Background(), domain.Alert{}); err == nil {
		t.Fatal("no notification service must fail")
	}
	broken := desktopnotify.New(func() (*dbus.Conn, error) { return nil, errors.New("no bus") }, time.Now)
	if err := broken.Notify(context.Background(), domain.Alert{}); err == nil {
		t.Fatal("no bus must fail")
	}
	if _, body := desktopnotify.Text(domain.Alert{Threshold: 90}, now); body != "Above 90 % of this window." {
		t.Fatalf("no reset = %q", body)
	}
}
