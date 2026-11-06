//go:build acceptance

package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/tray/traytest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit/systemdtest"
)

// collect → snapshot → tray: the panel shows what was collected, "Refresh
// now" asks systemd for the collector, and Quit ends the command.
func TestTheTrayShowsTheSnapshotAndAsksSystemdToRefresh(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	w.source.set(fiveHours(93, 2*time.Hour))
	w.collect()

	address := dbustest.Start(t)
	watcher := traytest.StartWatcher(t, address)
	manager := systemdtest.Start(t, address)
	w.vars["DBUS_SESSION_BUS_ADDRESS"] = address

	exited := make(chan int, 1)
	stderr := make(chan string, 1)
	go func() {
		code, _, errors := w.run("tray")
		stderr <- errors
		exited <- code
	}()
	panel := traytest.NewPanel(t, address, watcher.Registered(t))
	if status := panel.Property("Status").Value(); status != "NeedsAttention" {
		t.Fatalf("Status = %v at 93 %%", status)
	}
	var labels []string
	for _, e := range panel.Menu() {
		labels = append(labels, e.Label)
	}
	if !strings.HasPrefix(labels[0], "acme · PRO · 5h 93% ↻ 2h00m") || !strings.HasPrefix(labels[1], "zai · unsupported") {
		t.Fatalf("menu = %q", labels)
	}

	panel.Click("Refresh now")
	deadline := time.Now().Add(5 * time.Second)
	for !slices.Equal(manager.Started(), []string{"ataecina-collect.service replace"}) {
		if time.Now().After(deadline) {
			t.Fatalf("started = %v", manager.Started())
		}
		time.Sleep(20 * time.Millisecond)
	}

	panel.Click("Quit")
	select {
	case code := <-exited:
		if code != exitOK {
			t.Fatalf("tray = %d: %s", code, <-stderr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Quit did not end ataecina tray")
	}
}

func TestTheTrayNeedsADesktopSession(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	code, _, stderr := w.run("tray")
	if code != exitFailure || !strings.Contains(stderr, "no session bus") {
		t.Fatalf("tray = %d %q", code, stderr)
	}
	if code, _, _ := w.run("tray", "--unknown"); code != exitUsage {
		t.Fatalf("tray --unknown = %d", code)
	}
}
