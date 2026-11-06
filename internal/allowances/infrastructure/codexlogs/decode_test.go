package codexlogs_test

import (
	"bufio"
	"bytes"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlogs"
)

func fixtureLines(t testing.TB) [][]byte {
	t.Helper()
	data, err := os.ReadFile("testdata/rollout-synthetic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var lines [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, append([]byte(nil), scanner.Bytes()...))
	}
	return lines
}

func TestOnlyTokenCountEventsAreRead(t *testing.T) {
	lines := fixtureLines(t)
	for i, want := range []bool{false, false, true, true, false, true, false} {
		_, ok, err := codexlogs.DecodeLine(lines[i])
		if err != nil || ok != want {
			t.Fatalf("line %d: ok=%v err=%v, want ok=%v", i, ok, err, want)
		}
	}
}

func TestWindowsKeepTheirDurationWhateverTheirPosition(t *testing.T) {
	limits, _, err := codexlogs.DecodeLine(fixtureLines(t)[5])
	if err != nil {
		t.Fatal(err)
	}
	if limits.LimitID != "codex" || limits.Plan != "plus" || len(limits.Windows) != 2 {
		t.Fatalf("limits = %+v", limits)
	}
	if limits.Windows[0].Minutes != 10080 || limits.Windows[0].UsedPercent != 40 {
		t.Fatalf("primary = %+v", limits.Windows[0])
	}
	if !limits.Windows[1].ResetsAt.Equal(time.Date(2026, 11, 6, 15, 30, 0, 0, time.UTC)) {
		t.Fatalf("secondary resets at %v", limits.Windows[1].ResetsAt)
	}
	if limits.Credits == nil || limits.Credits.Balance != "4.25" || !limits.Credits.HasCredits {
		t.Fatalf("credits = %+v", limits.Credits)
	}
	if !limits.ObservedAt.Equal(time.Date(2026, 11, 6, 11, 30, 2, 0, time.UTC)) {
		t.Fatalf("observed at %v", limits.ObservedAt)
	}
}

func TestOlderLogsWithRelativeResetsAndNoLimitID(t *testing.T) {
	line := []byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":7,"window_minutes":300,"resets_in_seconds":3600}}}}`)
	limits, ok, err := codexlogs.DecodeLine(line)
	if err != nil || !ok {
		t.Fatalf("DecodeLine = %v, %v", ok, err)
	}
	if limits.LimitID != "codex" || !limits.Windows[0].ResetsAt.Equal(time.Date(2026, 11, 6, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("limits = %+v", limits)
	}
}

func TestMalformedRateLimitsAreReported(t *testing.T) {
	for _, line := range []string{
		`{"timestamp":"yesterday","type":"event_msg","payload":{"type":"token_count","rate_limits":{}}}`,
		`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":"full"}}`,
		`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":7}}}}`,
		`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":-7,"window_minutes":300}}}}`,
	} {
		if _, ok, err := codexlogs.DecodeLine([]byte(line)); err == nil || ok {
			t.Fatalf("DecodeLine(%s) = %v, %v", line, ok, err)
		}
	}
	if _, ok, err := codexlogs.DecodeLine([]byte(`not json "rate_limits"`)); ok || err != nil {
		t.Fatal("a line that is not JSON is simply skipped")
	}
}

// FuzzDecodeLine checks that no log line panics the decoder or yields
// windows that cannot exist.
func FuzzDecodeLine(f *testing.F) {
	for _, line := range fixtureLines(f) {
		f.Add(line)
	}
	f.Add([]byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":1e308,"window_minutes":1,"resets_at":-1}}}}`))
	f.Fuzz(func(t *testing.T, line []byte) {
		limits, ok, err := codexlogs.DecodeLine(line)
		if err != nil || !ok {
			return
		}
		for _, w := range limits.Windows {
			if w.Minutes <= 0 || w.UsedPercent < 0 {
				t.Fatalf("impossible window %+v from %q", w, line)
			}
		}
	})
}
