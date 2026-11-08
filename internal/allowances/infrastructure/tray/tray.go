// Package tray shows the snapshot as a StatusNotifierItem with a DBusMenu
// on the session bus, for the desktops without the Plasma widget (ADR
// 0007): XFCE, Cinnamon, MATE, LXQt, Budgie, Waybar and GNOME with the
// AppIndicator extension. Like every display it only reads the snapshot
// and never unseals a key; "Refresh now" asks systemd to start the
// collector (ADR 0005).
package tray

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
)

// The collector usually answers within a few seconds of "Refresh now".
const afterRefresh = 4 * time.Second

// Options are what the tray needs from the composition root.
type Options struct {
	// Report reads the snapshot (the report_snapshot use case).
	Report func() (domain.StatusReport, error)
	// Refresh starts the collector.
	Refresh func(ctx context.Context) error
	Now     func() time.Time
	// Every is how often the snapshot is read again.
	Every time.Duration
	// Warnings receives one line per problem the owner should know about.
	Warnings io.Writer
}

// Tray keeps the item and its menu in step with the snapshot.
type Tray struct {
	options Options
	menu    *menu
	actions chan action
	problem string
	status  string
	toolTip toolTip
}

// New prepares a tray; Run shows it.
func New(options Options) *Tray {
	actions := make(chan action, 1)
	return &Tray{options: options, actions: actions, menu: &menu{actions: actions}}
}

// Run shows the item until ctx ends or the owner chooses Quit. It keeps
// running without a StatusNotifierWatcher and registers when one appears.
func (t *Tray) Run(ctx context.Context, conn *dbus.Conn) error {
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("request %s: %w", name, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("the bus name %s is taken", name)
	}
	properties, err := export(conn, item{actions: t.actions}, t.menu)
	if err != nil {
		return err
	}

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	err = conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, watcherName),
	)
	if err != nil {
		return fmt.Errorf("watch for a StatusNotifierWatcher: %w", err)
	}

	t.update(conn, properties)
	registered := true
	if err := register(ctx, conn, name); err != nil {
		registered = false
		t.warn("no StatusNotifierWatcher on the session bus yet (%v); the icon appears when the panel provides one", err)
	}

	ticker := time.NewTicker(t.options.Every)
	defer ticker.Stop()
	var reread <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			t.update(conn, properties)
			// A watcher that appeared unseen (before the match took
			// effect) is found on the next tick.
			if !registered {
				registered = register(ctx, conn, name) == nil
			}
		case <-reread:
			reread = nil
			t.update(conn, properties)
		case chosen := <-t.actions:
			if chosen == quitAction {
				return nil
			}
			t.problem = ""
			if err := t.options.Refresh(ctx); err != nil {
				t.problem = "Refresh failed: " + err.Error()
				t.warn("refresh: %v", err)
			}
			t.update(conn, properties)
			reread = time.After(afterRefresh)
		case signal, open := <-signals:
			if !open {
				return errors.New("the session bus connection closed")
			}
			if watcherAppeared(signal) {
				err := register(ctx, conn, name)
				registered = err == nil
				if err != nil {
					t.warn("register with the StatusNotifierWatcher: %v", err)
				}
			}
		}
	}
}

func register(ctx context.Context, conn *dbus.Conn, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Object(watcherName, watcherPath).CallWithContext(ctx, watcherInterface+".RegisterStatusNotifierItem", 0, name).Err
}

// watcherAppeared reports a NameOwnerChanged giving the watcher a new owner.
func watcherAppeared(signal *dbus.Signal) bool {
	if signal.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(signal.Body) != 3 {
		return false
	}
	name, _ := signal.Body[0].(string)
	owner, _ := signal.Body[2].(string)
	return name == watcherName && owner != ""
}

// update reads the snapshot and refreshes the menu, the tooltip and the
// status, announcing what changed.
func (t *Tray) update(conn *dbus.Conn, properties *prop.Properties) {
	report, err := t.options.Report()
	now := t.options.Now()
	entries, lines := t.describe(report, err, now)
	t.menu.show(entries)

	status := "Active"
	if err == nil && terminal.Attention(report) {
		status = "NeedsAttention"
	}
	if status != t.status {
		t.status = status
		properties.SetMust(itemInterface, "Status", status)
		_ = conn.Emit(itemPath, itemInterface+".NewStatus", status)
	}
	tip := toolTip{IconName: iconName, IconPixmap: []pixmap{}, Title: "Ataecina", Description: escapeMarkup(strings.Join(lines, "\n"))}
	if tip.Description != t.toolTip.Description {
		t.toolTip = tip
		properties.SetMust(itemInterface, "ToolTip", tip)
		_ = conn.Emit(itemPath, itemInterface+".NewToolTip")
	}
}

// describe turns the report into menu entries and tooltip lines: one line
// per subscription (or why there is none), the last refresh problem, the
// snapshot's age, then the actions.
func (t *Tray) describe(report domain.StatusReport, err error, now time.Time) ([]entry, []string) {
	takenAt, taken := report.TakenAt()
	readable := err == nil && taken
	var lines []string
	switch {
	case err != nil:
		lines = []string{"The snapshot cannot be read: " + err.Error()}
	case !taken:
		lines = []string{terminal.NoSnapshot}
	default:
		for _, status := range report.Statuses() {
			lines = append(lines, terminal.Line(status, now))
		}
	}
	if len(lines) == 0 {
		lines = []string{"The snapshot lists no subscriptions."}
	}
	entries := make([]entry, 0, len(lines)+5)
	for i, line := range lines {
		entries = append(entries, entry{id: firstLineID + int32(i), label: menuLabel(line), enabled: readable})
	}
	if t.problem != "" {
		lines = append(lines, t.problem)
		entries = append(entries, entry{id: problemID, label: menuLabel(t.problem)})
	}
	entries = append(entries, entry{id: separatorID, separator: true})
	if readable {
		updated := "Ataecina · " + terminal.Updated(takenAt, now)
		lines = append([]string{updated}, lines...)
		entries = append(entries, entry{id: updatedID, label: menuLabel(updated)})
	}
	entries = append(entries,
		entry{id: refreshID, label: "Refresh now", enabled: true},
		entry{id: quitID, label: "Quit", enabled: true},
	)
	return entries, lines
}

// menuLabel keeps underscores literal: DBusMenu reads "_x" as a mnemonic.
func menuLabel(text string) string {
	return strings.ReplaceAll(text, "_", "__")
}

// escapeMarkup keeps the tooltip plain on hosts that render markup.
func escapeMarkup(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

func (t *Tray) warn(format string, args ...any) {
	if t.options.Warnings != nil {
		fmt.Fprintf(t.options.Warnings, "ataecina: tray: "+format+"\n", args...)
	}
}
