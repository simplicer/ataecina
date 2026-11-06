//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// codexLog writes a synthetic Codex session log under CODEX_HOME, with
// token_count events at the given instants (SPECS §3.2 schema only).
func (w *world) codexLog(events ...string) {
	w.t.Helper()
	home := filepath.Join(w.root, "codex")
	dir := filepath.Join(home, "sessions", "2026", "11", "06")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		w.t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-synthetic.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
		w.t.Fatal(err)
	}
	if err := os.Chtimes(path, w.clock.Now(), w.clock.Now()); err != nil {
		w.t.Fatal(err)
	}
	w.vars["CODEX_HOME"] = home
}

func codexEvent(at time.Time, primaryMinutes int, primaryUsed float64, secondaryMinutes int, secondaryUsed float64) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","plan_type":"plus","primary":{"used_percent":%v,"window_minutes":%d,"resets_at":%d},"secondary":{"used_percent":%v,"window_minutes":%d,"resets_at":%d},"credits":{"has_credits":true,"unlimited":false,"balance":"3"}}}}`,
		at.Format(time.RFC3339), primaryUsed, primaryMinutes, at.Add(time.Duration(primaryMinutes)*time.Minute).Unix(),
		secondaryUsed, secondaryMinutes, at.Add(time.Duration(secondaryMinutes)*time.Minute).Unix())
}

func TestCodexLimitsFromTheSessionLogs(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [{"id": "codex", "source": "codex-logs"}]}`)
	at := w.clock.Now().Add(-time.Minute)
	// The weekly window comes first on purpose: windows are mapped by duration.
	w.codexLog(codexEvent(at, 10080, 41, 300, 4))
	w.collect()

	doc := w.status()
	codex := doc.Data.Subscriptions[0]
	if codex.Outcome != "OK" || codex.Freshness != "FRESH" || len(codex.Meters) != 2 {
		t.Fatalf("codex = %+v", codex)
	}
	byWindow := map[string]float64{}
	for _, meter := range codex.Meters {
		byWindow[meter.Window] = meter.UsedPercent
	}
	if byWindow["FIVE_HOURS"] != 4 || byWindow["WEEKLY"] != 41 {
		t.Fatalf("meters = %+v", codex.Meters)
	}
	_, table, _ := w.run()
	if !strings.Contains(table, "PLUS") || !strings.Contains(table, "credits 3") {
		t.Fatalf("table:\n%s", table)
	}
}

func TestCodexWithoutLogsIsUnavailableNotBroken(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [{"id": "codex", "source": "codex-logs"}]}`)
	w.vars["CODEX_HOME"] = filepath.Join(w.root, "no-codex-here")
	w.collect()
	codex := w.status().Data.Subscriptions[0]
	if codex.Outcome != "UNAVAILABLE" || codex.ErrorClass == nil || *codex.ErrorClass != "UNREADABLE" {
		t.Fatalf("codex = %+v", codex)
	}
}
