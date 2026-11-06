//go:build acceptance

package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func (w *world) hook(input string, args ...string) (int, string, string) {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	env := w.env
	env.stdin, env.stdout, env.stderr = strings.NewReader(input), &stdout, &stderr
	code := run(context.Background(), append([]string{"hook", "claude-statusline"}, args...), env)
	return code, stdout.String(), stderr.String()
}

// statusLineInput imitates what Claude Code passes to its status line
// (synthetic; only the documented rate_limits matter).
func statusLineInput(now time.Time, fiveHour, sevenDay float64) string {
	return fmt.Sprintf(`{"session_id":"synthetic","model":{"id":"claude-synthetic"},"rate_limits":{"five_hour":{"used_percentage":%v,"resets_at":%d},"seven_day":{"used_percentage":%v,"resets_at":%d}}}`,
		fiveHour, now.Add(130*time.Minute).Unix(), sevenDay, now.Add(108*time.Hour).Unix())
}

const claudeConfig = `{"version": 1, "subscriptions": [{"id": "claude", "source": "claude-statusline", "plan": "PRO"}]}`

func TestTheStatusLineHookFeedsTheCollector(t *testing.T) {
	w := newWorld(t, claudeConfig)
	code, out, stderr := w.hook(statusLineInput(w.clock.Now(), 23.5, 41.2), "--print")
	if code != exitOK || strings.TrimSpace(out) != "5h 24% · 7d 41% · ↻ 2h10m" || stderr != "" {
		t.Fatalf("hook = %d %q %q", code, out, stderr)
	}
	w.collect()
	claude := w.status().Data.Subscriptions[0]
	if claude.Outcome != "OK" || len(claude.Meters) != 2 || claude.Meters[0].UsedPercent != 23.5 {
		t.Fatalf("claude = %+v", claude)
	}
	_, table, _ := w.run()
	if !strings.Contains(table, "PRO") || !strings.Contains(table, "5h 23.5% ↻ 2h10m") {
		t.Fatalf("table:\n%s", table)
	}
}

func TestAnIdleSessionCannotRewindTheLimits(t *testing.T) {
	w := newWorld(t, claudeConfig)
	w.hook(statusLineInput(w.clock.Now(), 60, 50))
	w.clock.advance(time.Minute)
	// An idle session re-draws its status line with what it saw earlier.
	_, out, _ := w.hook(statusLineInput(w.clock.Now().Add(-time.Minute), 20, 30), "--print")
	if !strings.HasPrefix(out, "5h 60% · 7d 50%") {
		t.Fatalf("summary after a replay = %q", out)
	}
}

func TestTheHookNeverFails(t *testing.T) {
	w := newWorld(t, claudeConfig)
	cases := [][]string{
		{"--print"}, // nothing known yet
		{"--print", "--subscription", "Bad Name"}, // invalid id
		{"--bogus"},          // usage error
		{"--print", "extra"}, // usage error
	}
	for _, args := range cases {
		if code, _, _ := w.hook("not json", args...); code != exitOK {
			t.Fatalf("hook %v = %d", args, code)
		}
	}
	if code, out, _ := w.hook("{}", "--print"); code != exitOK || strings.TrimSpace(out) != "5h – · 7d –" {
		t.Fatalf("no limits yet = %d %q", code, out)
	}
	if code, _, stderr := w.run("hook", "something-else"); code != exitOK || !strings.Contains(stderr, "usage") {
		t.Fatalf("unknown hook = %d %q", code, stderr)
	}
	big := `{"pad":"` + strings.Repeat("x", 2<<20) + `"}`
	if code, out, _ := w.hook(big, "--print"); code != exitOK || strings.TrimSpace(out) != "5h – · 7d –" {
		t.Fatalf("oversized input = %d %q", code, out)
	}
	delete(w.vars, "XDG_RUNTIME_DIR")
	if code, out, stderr := w.hook(statusLineInput(w.clock.Now(), 1, 1), "--print"); code != exitOK || strings.TrimSpace(out) != "5h – · 7d –" || stderr == "" {
		t.Fatalf("no runtime directory = %d %q %q", code, out, stderr)
	}
}

func TestClaudeWithoutTheHookExplainsWhatToDo(t *testing.T) {
	w := newWorld(t, claudeConfig)
	w.collect()
	_, table, _ := w.run()
	if !strings.Contains(table, "ataecina hook claude-statusline") {
		t.Fatalf("table:\n%s", table)
	}
}
