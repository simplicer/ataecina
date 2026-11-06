//go:build integration

package codexlogs_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlogs"
)

var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

type codexHome struct {
	t     testing.TB
	home  string
	state string
	log   string
}

// newCodexHome copies the synthetic fixture into a throw-away ~/.codex.
func newCodexHome(t testing.TB) *codexHome {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "codex")
	dir := filepath.Join(home, "sessions", "2026", "11", "06")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/rollout-synthetic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	c := &codexHome{t: t, home: home, state: filepath.Join(root, "state", "codex-logs"), log: filepath.Join(dir, "rollout-2026-11-06T09-59-58-synthetic.jsonl")}
	c.write(data)
	return c
}

func (c *codexHome) write(data []byte) {
	c.t.Helper()
	if err := os.WriteFile(c.log, data, 0o600); err != nil {
		c.t.Fatal(err)
	}
	c.touch(now)
}

func (c *codexHome) append(text string) {
	c.t.Helper()
	file, err := os.OpenFile(c.log, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		c.t.Fatal(err)
	}
	file.Close()
	c.touch(now)
}

func (c *codexHome) touch(at time.Time) {
	if err := os.Chtimes(c.log, at, at); err != nil {
		c.t.Fatal(err)
	}
}

func (c *codexHome) read() (domain.Reading, error) {
	source := codexlogs.New(c.home, c.state, func() time.Time { return now })
	return source.Read(context.Background(), domain.NewSubscription("codex", "openai", codexlogs.SourceID, domain.FidelityReported, ""))
}

func tokenCount(at time.Time, fiveHour, weekly float64) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","plan_type":"plus","primary":{"used_percent":%v,"window_minutes":300,"resets_at":%d},"secondary":{"used_percent":%v,"window_minutes":10080,"resets_at":%d}}}}`,
		at.Format(time.RFC3339), fiveHour, at.Add(3*time.Hour).Unix(), weekly, at.Add(100*time.Hour).Unix())
}

func used(t testing.TB, reading domain.Reading, window domain.Window) float64 {
	t.Helper()
	for _, meter := range reading.Meters() {
		if meter.Bucket() == "codex" && meter.Window() == window {
			return meter.Used().Float64()
		}
	}
	t.Fatalf("no %s meter", window.Kind())
	return 0
}

func TestTheLatestLimitsAreRead(t *testing.T) {
	c := newCodexHome(t)
	reading, err := c.read()
	if err != nil {
		t.Fatal(err)
	}
	if used(t, reading, domain.FiveHourWindow()) != 20 || used(t, reading, domain.WeeklyWindow()) != 40 {
		t.Fatalf("meters = %+v", reading.Meters())
	}
	if len(reading.Meters()) != 2 || reading.Plan() != "PLUS" || reading.Source() != codexlogs.SourceID {
		t.Fatalf("reading = %+v", reading)
	}
	if credits, ok := reading.Credits(); !ok || credits.Available().String() != "4.25" {
		t.Fatalf("credits = %+v", credits)
	}
	if at, _ := reading.ObservedAt(); !at.Equal(time.Date(2026, 11, 6, 11, 30, 2, 0, time.UTC)) {
		t.Fatalf("observed at %v", at)
	}
}

func TestLogsAreReadIncrementallyAndTheStateHoldsNoContent(t *testing.T) {
	c := newCodexHome(t)
	if _, err := c.read(); err != nil {
		t.Fatal(err)
	}
	c.append(tokenCount(now.Add(-time.Minute), 55, 60) + "\n")
	reading, err := c.read()
	if err != nil {
		t.Fatal(err)
	}
	if used(t, reading, domain.FiveHourWindow()) != 55 {
		t.Fatalf("appended line not read: %+v", reading.Meters())
	}
	state, err := os.ReadFile(filepath.Join(c.state, "codex.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"synthetic prompt", "synthetic answer", "/home/someone", "input_tokens"} {
		if bytes.Contains(state, []byte(secret)) {
			t.Fatalf("the state file keeps log content: %q", secret)
		}
	}
	if info, _ := os.Stat(filepath.Join(c.state, "codex.json")); info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %04o", info.Mode().Perm())
	}
}

func TestALineStillBeingWrittenWaits(t *testing.T) {
	c := newCodexHome(t)
	line := tokenCount(now.Add(-time.Minute), 77, 60)
	c.append(line[:len(line)/2])
	reading, _ := c.read()
	if used(t, reading, domain.FiveHourWindow()) != 20 {
		t.Fatal("half a line must not be read")
	}
	c.append(line[len(line)/2:] + "\n")
	reading, _ = c.read()
	if used(t, reading, domain.FiveHourWindow()) != 77 {
		t.Fatal("the completed line must be read")
	}
}

func TestLongLinesAreSkipped(t *testing.T) {
	c := newCodexHome(t)
	c.append(`{"timestamp":"2026-11-06T11:59:00Z","type":"response_item","payload":{"text":"` + strings.Repeat("x", 100<<10) + `","rate_limits":1}}` + "\n")
	c.append(tokenCount(now.Add(-30*time.Second), 66, 60) + "\n")
	reading, err := c.read()
	if err != nil || used(t, reading, domain.FiveHourWindow()) != 66 {
		t.Fatalf("reading after a long line = %v", err)
	}
}

func TestARewrittenFileIsReadAgain(t *testing.T) {
	c := newCodexHome(t)
	if _, err := c.read(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.log); err != nil {
		t.Fatal(err)
	}
	c.write([]byte(tokenCount(now.Add(-time.Minute), 9, 60) + "\n"))
	reading, err := c.read()
	if err != nil || used(t, reading, domain.FiveHourWindow()) != 9 {
		t.Fatalf("rewritten file = %v", err)
	}
}

func TestOlderLogsAreReadOnceWhenRecentOnesLackCodexFigures(t *testing.T) {
	c := newCodexHome(t)
	c.touch(now.Add(-10 * 24 * time.Hour))
	// A recent log with only a bucket that carries nothing, as Codex
	// writes for `premium` when the plan has no such limit.
	recent := filepath.Join(filepath.Dir(c.log), "rollout-recent.jsonl")
	premium := `{"timestamp":"2026-11-06T11:59:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"premium","primary":null,"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":"0"}}}}` + "\n"
	if err := os.WriteFile(recent, []byte(premium), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now, now); err != nil {
		t.Fatal(err)
	}
	reading, err := c.read()
	if err != nil || used(t, reading, domain.WeeklyWindow()) != 40 || reading.Plan() != "PLUS" {
		t.Fatalf("older logs = %v", err)
	}
	// The backfill happens once: its figures stay in the state even if the
	// old file disappears.
	if err := os.Remove(c.log); err != nil {
		t.Fatal(err)
	}
	if reading, err := c.read(); err != nil || used(t, reading, domain.WeeklyWindow()) != 40 {
		t.Fatalf("after the backfill = %v", err)
	}
}

func TestAnEventWithNullWindowsKeepsTheKnownOnes(t *testing.T) {
	c := newCodexHome(t)
	// Codex sometimes logs its own bucket with null windows and credits.
	c.append(`{"timestamp":"2026-11-06T11:45:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":null,"secondary":null,"credits":null,"plan_type":null}}}` + "\n")
	reading, err := c.read()
	if err != nil || used(t, reading, domain.FiveHourWindow()) != 20 || reading.Plan() != "PLUS" {
		t.Fatalf("null windows erased the known ones: %v", err)
	}
	if at, _ := reading.ObservedAt(); !at.Equal(time.Date(2026, 11, 6, 11, 30, 2, 0, time.UTC)) {
		t.Fatalf("observed_at must follow the windows, got %v", at)
	}
}

func TestTheBackfillLooksPastSnapshotsWithoutWindows(t *testing.T) {
	c := newCodexHome(t)
	c.touch(now.Add(-12 * 24 * time.Hour))
	nulls := filepath.Join(filepath.Dir(c.log), "rollout-nulls.jsonl")
	empty := `{"timestamp":"2026-11-06T11:50:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":null,"secondary":null,"credits":null}}}` + "\n"
	if err := os.WriteFile(nulls, []byte(empty), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(nulls, now.Add(-10*24*time.Hour), now.Add(-10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	reading, err := c.read()
	if err != nil || used(t, reading, domain.WeeklyWindow()) != 40 {
		t.Fatalf("the backfill stopped at a snapshot without windows: %v", err)
	}
}

func TestNoRateLimitsAnywhere(t *testing.T) {
	c := newCodexHome(t)
	c.write([]byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"session_meta","payload":{}}` + "\n"))
	c.touch(now.Add(-30 * 24 * time.Hour))
	if _, err := c.read(); !hasClass(err, domain.ErrorUnreadable) {
		t.Fatalf("no rate limits = %v", err)
	}
}

func TestMissingStaleOrChangedLogs(t *testing.T) {
	c := newCodexHome(t)
	c.write([]byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":"high"}}}}` + "\n"))
	if _, err := c.read(); !hasClass(err, domain.ErrorContractChanged) {
		t.Fatalf("changed format = %v", err)
	}

	c = newCodexHome(t)
	c.write([]byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex"}}}` + "\n"))
	if _, err := c.read(); !hasClass(err, domain.ErrorUnreadable) {
		t.Fatalf("no figures = %v", err)
	}

	c = newCodexHome(t)
	c.write([]byte(`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":20000,"window_minutes":300}}}}` + "\n"))
	if _, err := c.read(); !hasClass(err, domain.ErrorContractChanged) {
		t.Fatalf("out of range = %v", err)
	}

	missing := codexlogs.New(filepath.Join(t.TempDir(), "absent"), t.TempDir(), func() time.Time { return now })
	if _, err := missing.Read(context.Background(), domain.NewSubscription("codex", "openai", codexlogs.SourceID, domain.FidelityReported, "")); !hasClass(err, domain.ErrorUnreadable) {
		t.Fatalf("no ~/.codex = %v", err)
	}

	c = newCodexHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := codexlogs.New(c.home, c.state, func() time.Time { return now })
	if _, err := source.Read(ctx, domain.NewSubscription("codex", "openai", codexlogs.SourceID, domain.FidelityReported, "")); !hasClass(err, domain.ErrorUnavailable) {
		t.Fatalf("cancelled = %v", err)
	}
}

func TestACorruptStateIsForgotten(t *testing.T) {
	c := newCodexHome(t)
	if err := os.MkdirAll(c.state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.state, "codex.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reading, err := c.read(); err != nil || used(t, reading, domain.FiveHourWindow()) != 20 {
		t.Fatalf("corrupt state = %v", err)
	}
}

func hasClass(err error, class domain.ErrorClass) bool {
	var failure *domain.SourceFailure
	return errors.As(err, &failure) && failure.Class() == class
}

// BenchmarkIncrementalScan measures a collection when Codex wrote nothing
// new: the reader only checks offsets.
func BenchmarkIncrementalScan(b *testing.B) {
	c := newCodexHome(b)
	var lines strings.Builder
	for i := range 5000 {
		lines.WriteString(tokenCount(now.Add(-time.Duration(5000-i)*time.Second), float64(i%100), 50) + "\n")
	}
	c.write([]byte(lines.String()))
	if _, err := c.read(); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := c.read(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFirstScan measures reading a large session log from the start.
func BenchmarkFirstScan(b *testing.B) {
	c := newCodexHome(b)
	var lines strings.Builder
	for i := range 5000 {
		lines.WriteString(tokenCount(now.Add(-time.Duration(5000-i)*time.Second), float64(i%100), 50) + "\n")
	}
	c.write([]byte(lines.String()))
	for b.Loop() {
		_ = os.RemoveAll(c.state)
		if _, err := c.read(); err != nil {
			b.Fatal(err)
		}
	}
}
