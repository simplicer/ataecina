//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// syntheticCodexLog writes a Codex session log built from the documented
// schema (SPECS §3.2), never from a real log.
func syntheticCodexLog(t *testing.T, home string, at time.Time) {
	t.Helper()
	dir := filepath.Join(home, "sessions", at.Format("2006"), at.Format("01"), at.Format("02"))
	mustPrivateDir(t, dir)
	lines := fmt.Sprintf(`{"timestamp":%q,"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"synthetic"}]}}
{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","plan_type":"plus","primary":{"used_percent":41,"window_minutes":10080,"resets_at":%d},"secondary":{"used_percent":4,"window_minutes":300,"resets_at":%d},"credits":{"has_credits":true,"unlimited":false,"balance":"0"}}}}
`, at.Format(time.RFC3339), at.Format(time.RFC3339), at.Add(100*time.Hour).Unix(), at.Add(3*time.Hour).Unix())
	if err := os.WriteFile(filepath.Join(dir, "rollout-synthetic.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCodexLogsEndToEnd(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "codex", "source": "codex-logs"}]}`, nil)
	codexHome := filepath.Join(tr.root, "home", ".codex")
	syntheticCodexLog(t, codexHome, time.Now().UTC().Add(-time.Minute))

	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	code, out, stderr := tr.run(bin, "status", "--json")
	if code != 0 {
		t.Fatalf("status = %d: %s", code, stderr)
	}
	var doc struct {
		Data struct {
			Subscriptions []struct {
				Outcome string `json:"outcome"`
				Plan    string `json:"plan"`
				Source  string `json:"source"`
				Meters  []struct {
					Bucket      string  `json:"bucket"`
					Window      string  `json:"window"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"meters"`
				Credits *struct {
					Available float64 `json:"available"`
				} `json:"credits"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	codex := doc.Data.Subscriptions[0]
	if codex.Outcome != "OK" || codex.Plan != "PLUS" || codex.Source != "CODEX_LOGS" || codex.Credits == nil {
		t.Fatalf("codex = %+v", codex)
	}
	if codex.Meters[0].Window != "WEEKLY" || codex.Meters[0].UsedPercent != 41 || codex.Meters[1].Window != "FIVE_HOURS" || codex.Meters[0].Bucket != "codex" {
		t.Fatalf("meters = %+v", codex.Meters)
	}
	if _, err := os.Stat(filepath.Join(tr.root, "state", "ataecina", "codex-logs", "codex.json")); err != nil {
		t.Fatalf("offsets not kept: %v", err)
	}
}
