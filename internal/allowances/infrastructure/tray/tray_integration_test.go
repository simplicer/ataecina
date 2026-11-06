//go:build integration

package tray_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/tray"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/tray/traytest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

var taken = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func busyReport(t *testing.T) domain.StatusReport {
	t.Helper()
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "zai", Provider: "zai", Source: "zai-api", Plan: "LITE",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), 92, taken.Add(41*time.Minute))},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, []domain.Reading{reading})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.StatusAt(taken)
}

// syncBuffer is a goroutine-safe warnings sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestThePanelShowsTheTrayAndItsMenuActs(t *testing.T) {
	address := dbustest.Start(t)
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	refreshes := make(chan struct{}, 4)
	var asked atomic.Int32
	var warnings syncBuffer
	tr := tray.New(tray.Options{
		Report: func() (domain.StatusReport, error) { return busyReport(t), nil },
		Refresh: func(context.Context) error {
			refreshes <- struct{}{}
			if asked.Add(1) > 1 {
				return errors.New("systemd said no")
			}
			return nil
		},
		Now:      func() time.Time { return taken.Add(time.Minute) },
		Every:    time.Hour,
		Warnings: &warnings,
	})
	done := make(chan error, 1)
	go func() { done <- tr.Run(context.Background(), conn) }()

	// The tray starts before the panel: it waits and registers when a
	// watcher appears.
	time.Sleep(200 * time.Millisecond)
	watcher := traytest.StartWatcher(t, address)
	service := watcher.Registered(t)
	if !strings.HasPrefix(service, "org.kde.StatusNotifierItem-") {
		t.Fatalf("registered %q", service)
	}
	if !strings.Contains(warnings.String(), "no StatusNotifierWatcher") {
		t.Fatalf("warnings = %q", warnings.String())
	}

	panel := traytest.NewPanel(t, address, service)
	for name, want := range map[string]any{"Id": "ataecina", "IconName": "ataecina", "Status": "NeedsAttention", "ItemIsMenu": true, "Category": "ApplicationStatus"} {
		if got := panel.Property(name).Value(); got != want {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	tip, ok := panel.Property("ToolTip").Value().([]any)
	if !ok || len(tip) != 4 || tip[2] != "Ataecina" || !strings.Contains(tip[3].(string), "zai · LITE · 5h 92% ↻ 40m") {
		t.Fatalf("ToolTip = %#v", panel.Property("ToolTip").Value())
	}

	menu := panel.Menu()
	var shown []string
	for _, e := range menu {
		shown = append(shown, e.Label)
	}
	want := "zai · LITE · 5h 92% ↻ 40m · fresh 1m ago||Ataecina · updated 1m ago|Refresh now|Quit"
	if strings.Join(shown, "|") != want || !menu[1].Separator || menu[2].Enabled || !menu[3].Enabled {
		t.Fatalf("menu = %+v", menu)
	}

	panel.Click("Refresh now")
	waitFor(t, refreshes)
	panel.MiddleClick()
	waitFor(t, refreshes)
	deadline := time.Now().Add(5 * time.Second)
	for !contains(panel.Menu(), "Refresh failed: systemd said no") {
		if time.Now().After(deadline) {
			t.Fatalf("the failed refresh is not shown: %+v", panel.Menu())
		}
		time.Sleep(20 * time.Millisecond)
	}

	panel.Click("Quit")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Quit did not stop the tray")
	}
}

func TestTheTrayStopsWithItsContext(t *testing.T) {
	address := dbustest.Start(t)
	traytest.StartWatcher(t, address)
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	tr := tray.New(tray.Options{
		Report:  func() (domain.StatusReport, error) { return domain.StatusReport{}, nil },
		Refresh: func(context.Context) error { return nil },
		Now:     time.Now,
		Every:   10 * time.Millisecond,
	})
	done := make(chan error, 1)
	go func() { done <- tr.Run(ctx, conn) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tray ignored its context")
	}
}

func waitFor(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("no refresh")
	}
}

func contains(menu []traytest.Entry, label string) bool {
	for _, e := range menu {
		if e.Label == label {
			return true
		}
	}
	return false
}
