//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func (tr *tree) runWithInput(bin, input string, args ...string) (int, string, string, time.Duration) {
	tr.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = tr.env
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	err := cmd.Run()
	elapsed := time.Since(started)
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		tr.t.Fatal(err)
	}
	return code, stdout.String(), stderr.String(), elapsed
}

func TestTheClaudeStatusLineHookEndToEnd(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "claude", "source": "claude-statusline", "plan": "PRO"}]}`, nil)
	now := time.Now().UTC()
	input := fmt.Sprintf(`{"session_id":"synthetic","model":{"id":"claude-synthetic"},"rate_limits":{"five_hour":{"used_percentage":23,"resets_at":%d},"seven_day":{"used_percentage":41,"resets_at":%d}}}`,
		now.Add(131*time.Minute).Unix(), now.Add(100*time.Hour).Unix())

	code, out, stderr, elapsed := tr.runWithInput(bin, input, "hook", "claude-statusline", "--print")
	if code != 0 || !strings.HasPrefix(out, "5h 23% · 7d 41% · ↻ 2h1") || stderr != "" {
		t.Fatalf("hook = %d %q %q", code, out, stderr)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("the hook took %v", elapsed)
	}
	t.Logf("hook took %v", elapsed)

	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	_, status, _ := tr.run(bin, "status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []struct {
				Outcome string `json:"outcome"`
				Source  string `json:"source"`
				Plan    string `json:"plan"`
				Meters  []struct {
					UsedPercent float64 `json:"used_percent"`
				} `json:"meters"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status), &doc); err != nil {
		t.Fatal(err)
	}
	claude := doc.Data.Subscriptions[0]
	if claude.Outcome != "OK" || claude.Source != "CLAUDE_STATUSLINE" || claude.Plan != "PRO" || claude.Meters[1].UsedPercent != 41 {
		t.Fatalf("claude = %+v", claude)
	}
}

func TestTheHookExitsZeroOnGarbage(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1}`, nil)
	code, out, _, _ := tr.runWithInput(bin, "\x00garbage", "hook", "claude-statusline", "--print")
	if code != 0 || strings.TrimSpace(out) != "5h – · 7d –" {
		t.Fatalf("hook = %d %q", code, out)
	}
}
