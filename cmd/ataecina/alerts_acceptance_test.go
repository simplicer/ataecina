//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify/notifytest"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

func TestAThresholdIsAnnouncedOncePerWindow(t *testing.T) {
	w := newWorld(t, `{"version": 1, "alerts": {"thresholds": [90, 95]}, "subscriptions": [{"id": "acme", "source": "scripted"}]}`)
	address := dbustest.Start(t)
	desktop := notifytest.Start(t, address)
	w.vars["DBUS_SESSION_BUS_ADDRESS"] = address

	w.source.set(fiveHours(91, 2*time.Hour))
	w.collect()
	w.clock.advance(5 * time.Minute)
	w.source.set(fiveHours(93, 2*time.Hour-5*time.Minute))
	w.collect()
	if shown := desktop.Shown(); len(shown) != 1 || !strings.HasPrefix(shown[0].Summary, "acme · five hours at 91") {
		t.Fatalf("shown = %+v", shown)
	}
	w.clock.advance(5 * time.Minute)
	w.source.set(fiveHours(97, 2*time.Hour-10*time.Minute))
	w.collect()
	if shown := desktop.Shown(); len(shown) != 2 || shown[1].Urgency != 2 {
		t.Fatalf("crossing 95 = %+v", shown)
	}
	// The window is reborn: the next one announces again.
	w.clock.advance(3 * time.Hour)
	w.source.set(fiveHours(92, 5*time.Hour))
	w.collect()
	if shown := desktop.Shown(); len(shown) != 3 {
		t.Fatalf("after rebirth = %+v", shown)
	}
	state, _ := os.ReadFile(filepath.Join(w.root, "state", "ataecina", "alerts.json"))
	if strings.Count(string(state), `"threshold"`) != 1 {
		t.Fatalf("only the current window is remembered:\n%s", state)
	}
}

func TestAFailingSourceIsAnnouncedOnceUntilItRecovers(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [{"id": "acme", "source": "scripted"}]}`)
	address := dbustest.Start(t)
	desktop := notifytest.Start(t, address)
	w.vars["DBUS_SESSION_BUS_ADDRESS"] = address

	w.source.set(func(domain.Subscription, time.Time) (domain.Reading, error) {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected shape", nil)
	})
	w.collect()
	w.clock.advance(5 * time.Minute)
	w.collect()
	if shown := desktop.Shown(); len(shown) != 1 || shown[0].Summary != "acme · broken" {
		t.Fatalf("shown = %+v", shown)
	}
	state := filepath.Join(w.root, "state", "ataecina", "alerts.json")
	if kept, _ := os.ReadFile(state); !strings.Contains(string(kept), `"failure": "BROKEN"`) {
		t.Fatalf("the failure is remembered:\n%s", kept)
	}
	w.clock.advance(5 * time.Minute)
	w.source.set(fiveHours(10, time.Hour))
	w.collect()
	if kept, _ := os.ReadFile(state); strings.Contains(string(kept), `"failure"`) || len(desktop.Shown()) != 1 {
		t.Fatalf("a source that recovers is forgotten, silently:\n%s", kept)
	}
	logged, _ := os.ReadFile(filepath.Join(w.root, "state", "ataecina", "ataecina.log"))
	if strings.Count(string(logged), `"msg":"source failing"`) != 1 {
		t.Fatalf("log:\n%s", logged)
	}
}

func TestWithoutADesktopAlertsAreLogged(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [{"id": "acme", "source": "scripted", "thresholds": [50]}]}`)
	w.source.set(fiveHours(60, time.Hour))
	w.collect()
	logged, _ := os.ReadFile(filepath.Join(w.root, "state", "ataecina", "ataecina.log"))
	if !strings.Contains(string(logged), `"msg":"alert raised"`) || !strings.Contains(string(logged), `"threshold":50`) || !strings.Contains(string(logged), "no session bus") {
		t.Fatalf("log:\n%s", logged)
	}
}
