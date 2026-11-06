//go:build integration

package snapshotfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/snapshotfile"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

var taken = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func sampleSnapshot(t testing.TB) domain.Snapshot {
	t.Helper()
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline", Plan: "PRO",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), 23.5, taken.Add(2*time.Hour))},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, []domain.Reading{reading})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestTheSnapshotSurvivesARoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run", "ataecina")
	store := snapshotfile.NewStore(dir)
	if _, err := store.LatestSnapshot(); !errors.Is(err, domain.ErrNoSnapshot) {
		t.Fatalf("before the first collection: %v", err)
	}
	if err := store.KeepSnapshot(sampleSnapshot(t)); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(store.Path())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o", info.Mode().Perm())
	}
	dirInfo, _ := os.Stat(dir)
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %04o", dirInfo.Mode().Perm())
	}
	snapshot, err := store.LatestSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	reading, ok := snapshot.ReadingOf("claude")
	if !ok || reading.Meters()[0].Used() != 23.5 || snapshot.Interval() != 5*time.Minute {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestACorruptOrLooseSnapshotIsRefused(t *testing.T) {
	cases := map[string]string{
		"not json":     `{`,
		"old format":   `{"version":7}`,
		"bad reading":  `{"version":1,"taken_at":"2026-11-06T19:40:00Z","interval_seconds":300,"readings":[{"subscription":"Bad"}]}`,
		"bad interval": `{"version":1,"taken_at":"2026-11-06T19:40:00Z","interval_seconds":0,"readings":[]}`,
	}
	for name, content := range cases {
		dir := t.TempDir()
		store := snapshotfile.NewStore(dir)
		if err := privatefs.WriteFile(store.Path(), []byte(content)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.LatestSnapshot(); !errors.Is(err, snapshotfile.ErrCorrupt) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	dir := filepath.Join(t.TempDir(), "run")
	store := snapshotfile.NewStore(dir)
	if err := store.KeepSnapshot(sampleSnapshot(t)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LatestSnapshot(); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("loose snapshot: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.KeepSnapshot(sampleSnapshot(t)); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("loose directory: %v", err)
	}
}

func BenchmarkLatestSnapshot(b *testing.B) {
	store := snapshotfile.NewStore(filepath.Join(b.TempDir(), "run"))
	if err := store.KeepSnapshot(sampleSnapshot(b)); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := store.LatestSnapshot(); err != nil {
			b.Fatal(err)
		}
	}
}
