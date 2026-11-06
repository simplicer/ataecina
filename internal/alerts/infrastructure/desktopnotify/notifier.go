// Package desktopnotify shows alerts through org.freedesktop.Notifications
// on the session bus (Plasma, GNOME, XFCE and every other desktop).
package desktopnotify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

// D-Bus names of the notification service.
const (
	BusName    = "org.freedesktop.Notifications"
	ObjectPath = dbus.ObjectPath("/org/freedesktop/Notifications")
	method     = "org.freedesktop.Notifications.Notify"
	appName    = "Ataecina"
	appIcon    = "ataecina"
)

// Notifier sends one notification per alert.
type Notifier struct {
	connect func() (*dbus.Conn, error)
	now     func() time.Time
}

// New connects with connect for each notification.
func New(connect func() (*dbus.Conn, error), now func() time.Time) Notifier {
	return Notifier{connect: connect, now: now}
}

// Notify shows "zai · 5 hours at 92 %" with the time to rebirth; the
// second threshold and above are critical.
func (n Notifier) Notify(ctx context.Context, alert domain.Alert) error {
	conn, err := n.connect()
	if err != nil {
		return fmt.Errorf("no session bus: %w", err)
	}
	defer conn.Close()
	summary, body := Text(alert, n.now())
	urgency := byte(1)
	if alert.Threshold >= 95 {
		urgency = 2
	}
	hints := map[string]dbus.Variant{
		"urgency":       dbus.MakeVariant(urgency),
		"category":      dbus.MakeVariant("ataecina.quota"),
		"desktop-entry": dbus.MakeVariant("org.simplicer.ataecina"),
	}
	var id uint32
	return conn.Object(BusName, ObjectPath).CallWithContext(ctx, method, 0,
		appName, uint32(0), appIcon, summary, body, []string{}, hints, int32(-1)).Store(&id)
}

// Text is the summary and body of an alert.
func Text(alert domain.Alert, now time.Time) (string, string) {
	window := strings.ToLower(strings.ReplaceAll(alert.Instance.Window, "_", " "))
	if alert.Instance.Bucket != "" {
		window = alert.Instance.Bucket + " " + window
	}
	summary := fmt.Sprintf("%s · %s at %.0f %%", alert.Instance.Subscription, window, alert.UsedPercent)
	body := fmt.Sprintf("Above %d %% of this window.", alert.Threshold)
	if reset := alert.Instance.ResetsAt; !reset.IsZero() && reset.After(now) {
		body += " Reborn in " + reset.Sub(now).Round(time.Minute).String() + "."
	}
	return summary, body
}
