//go:build integration

package opencodezen_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodezen"
)

// fakeOpencode points at tests/e2e/testdata/fake-opencode, a shell script
// that prints the given output; the real opencode is never run.
func fakeOpencode(t *testing.T, output string) (opencodezen.Stats, string) {
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
	return opencodezen.NewStats(func() (string, error) { return script, nil }), journal
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
	missing := opencodezen.NewStats(func() (string, error) { return "", errors.New("not found") })
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
