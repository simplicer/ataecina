//go:build integration

package opencodezen_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodezen"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// fakeOpencode points at tests/e2e/testdata/fake-opencode, a shell script
// that prints the given output; the real opencode is never run.
func fakeOpencode(t *testing.T, output string) (opencodezen.Stats, string) {
	t.Helper()
	return keptOpencode(t, output, nil)
}

// keptOpencode is fakeOpencode with its answers kept by reuse.
func keptOpencode(t *testing.T, output string, reuse *officialcli.Reuse) (opencodezen.Stats, string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "tests", "e2e", "testdata", "fake-opencode"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	stats := filepath.Join(dir, "stats.json")
	if err := os.WriteFile(stats, []byte(output), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(dir, "journal")
	t.Setenv("FAKE_OPENCODE_STATS", stats)
	t.Setenv("FAKE_OPENCODE_JOURNAL", journal)
	return opencodezen.NewStats(func() (string, error) { return script, nil }, reuse), journal
}

func TestTheSpendingIsReusedUntilOpencodeIsUsedAgain(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "opencode.db")
	if err := os.WriteFile(database, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	reuse := &officialcli.Reuse{File: filepath.Join(dir, "state", "opencode-stats.json"), Watch: []string{database}, MaxAge: time.Hour}
	stats, journal := keptOpencode(t, string(fixture(t)), reuse)
	for range 2 {
		if spent, err := stats.SpentToDate(context.Background()); err != nil || spent.Amount().String() != "4.75" {
			t.Fatalf("SpentToDate = %v, %v", spent, err)
		}
	}
	if runs := lines(t, journal); runs != 1 {
		t.Fatalf("opencode ran %d times, want once", runs)
	}
	kept, err := os.ReadFile(reuse.File)
	if err != nil || !strings.Contains(string(kept), `"spent_usd":"4.75"`) || strings.Contains(string(kept), "models") {
		t.Fatalf("kept = %s, %v; want the spend only", kept, err)
	}

	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(database, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := stats.SpentToDate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runs := lines(t, journal); runs != 2 {
		t.Fatalf("opencode ran %d times after it was used, want twice", runs)
	}
}

func TestAnUnexpectedAnswerIsNeverKept(t *testing.T) {
	reuse := &officialcli.Reuse{File: filepath.Join(t.TempDir(), "state", "opencode-stats.json"), MaxAge: time.Hour}
	stats, journal := keptOpencode(t, `{"totals": {}}`, reuse)
	for range 2 {
		if _, err := stats.SpentToDate(context.Background()); err == nil {
			t.Fatal("an unexpected answer was accepted")
		}
	}
	if runs := lines(t, journal); runs != 2 {
		t.Fatalf("opencode ran %d times, want twice", runs)
	}
}

func lines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

func TestTheSpendingComesFromAFixedCommand(t *testing.T) {
	stats, journal := fakeOpencode(t, string(fixture(t)))
	spent, err := stats.SpentToDate(context.Background())
	if err != nil || spent.Amount().String() != "4.75" || spent.Currency() != "USD" {
		t.Fatalf("SpentToDate = %v, %v", spent, err)
	}
	argv, _ := os.ReadFile(journal)
	if strings.TrimSpace(string(argv)) != "stats --all --models --limit 1000 --json --standalone" {
		t.Fatalf("argv = %q", argv)
	}
}

func TestStatsFailuresAreClassified(t *testing.T) {
	var failure *domain.SourceFailure
	missing := opencodezen.NewStats(func() (string, error) { return "", errors.New("not found") }, nil)
	if _, err := missing.SpentToDate(context.Background()); !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("missing opencode = %v", err)
	}
	stats, _ := fakeOpencode(t, `{"totals": {}}`)
	if _, err := stats.SpentToDate(context.Background()); !errors.As(err, &failure) || failure.Class() != domain.ErrorContractChanged {
		t.Fatalf("unexpected output = %v", err)
	}
	stats, _ = fakeOpencode(t, `{"models": [], "pad": "`+strings.Repeat("x", 2<<20)+`"}`)
	if _, err := stats.SpentToDate(context.Background()); !errors.As(err, &failure) || failure.Class() != domain.ErrorContractChanged {
		t.Fatalf("oversized output = %v", err)
	}
	stats, _ = fakeOpencode(t, `{"models": []}`)
	t.Setenv("FAKE_OPENCODE_FAIL", "1")
	if _, err := stats.SpentToDate(context.Background()); !errors.As(err, &failure) || failure.Class() != domain.ErrorUnavailable {
		t.Fatalf("failing opencode = %v", err)
	}
}
