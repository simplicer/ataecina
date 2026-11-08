// Package desktopnotify shows alerts through org.freedesktop.Notifications
// on the session bus (Plasma, GNOME, XFCE and every other desktop).
package desktopnotify

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/locale"
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
	connect  func() (*dbus.Conn, error)
	now      func() time.Time
	language locale.Language
}

// New connects with connect for each notification and writes in the
// session's language.
func New(connect func() (*dbus.Conn, error), now func() time.Time, language locale.Language) Notifier {
	return Notifier{connect: connect, now: now, language: language}
}

// Notify shows "zai · 5 hours at 92%" with the time to rebirth; the
// second threshold and above are critical.
func (n Notifier) Notify(ctx context.Context, alert domain.Alert) error {
	summary, body := Text(alert, n.now(), n.language)
	urgency := byte(1)
	if alert.Threshold >= 95 {
		urgency = 2
	}
	return send(ctx, n.connect, summary, body, urgency, "ataecina.quota")
}

// FailureNotifier sends one notification per source that starts failing.
type FailureNotifier struct {
	connect  func() (*dbus.Conn, error)
	language locale.Language
}

// NewFailureNotifier connects with connect for each notification and
// writes in the session's language.
func NewFailureNotifier(connect func() (*dbus.Conn, error), language locale.Language) FailureNotifier {
	return FailureNotifier{connect: connect, language: language}
}

// NotifySourceFailure shows "workers-ai · key missing" with the source's
// hint.
func (n FailureNotifier) NotifySourceFailure(ctx context.Context, alert domain.SourceAlert) error {
	summary, body := FailureText(alert, n.language)
	return send(ctx, n.connect, summary, body, 1, "ataecina.source")
}

// FailureText is the summary and body of a failing source.
func FailureText(alert domain.SourceAlert, language locale.Language) (string, string) {
	words := wordingFor(language)
	summary := alert.Subscription + " · " + words.failure(alert.Failure)
	if alert.Hint == "" {
		return summary, words.unreadable
	}
	return summary, alert.Hint
}

func send(ctx context.Context, connect func() (*dbus.Conn, error), summary, body string, urgency byte, category string) error {
	conn, err := connect()
	if err != nil {
		return fmt.Errorf("no session bus: %w", err)
	}
	defer conn.Close()
	hints := map[string]dbus.Variant{
		"urgency":       dbus.MakeVariant(urgency),
		"category":      dbus.MakeVariant(category),
		"desktop-entry": dbus.MakeVariant("org.simplicer.ataecina"),
	}
	var id uint32
	return conn.Object(BusName, ObjectPath).CallWithContext(ctx, method, 0,
		appName, uint32(0), appIcon, summary, body, []string{}, hints, int32(-1)).Store(&id)
}

// Text is the summary and body of an alert, in the language given.
func Text(alert domain.Alert, now time.Time, language locale.Language) (string, string) {
	words := wordingFor(language)
	window := words.window(alert.Instance.Window)
	if alert.Instance.Bucket != "" {
		window = alert.Instance.Bucket + " " + window
	}
	used := fmt.Sprintf(words.percent, int(math.Round(alert.UsedPercent)))
	summary := fmt.Sprintf(words.summary, alert.Instance.Subscription, window, used)
	body := fmt.Sprintf(words.above, fmt.Sprintf(words.percent, alert.Threshold))
	if reset := alert.Instance.ResetsAt; !reset.IsZero() && reset.After(now) {
		body += " " + fmt.Sprintf(words.reborn, words.duration(reset.Sub(now)))
	}
	return summary, body
}
