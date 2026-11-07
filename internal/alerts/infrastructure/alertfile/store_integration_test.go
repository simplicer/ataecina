//go:build integration

package alertfile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/alertfile"
)

func TestTheStateRoundTrips(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "ataecina")
	store := alertfile.NewStore(dir)
	empty, err := store.Load()
	if err != nil || len(empty.Raised()) != 0 {
		t.Fatalf("first load = %v", err)
	}
	at := time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)
	state := domain.NewAlertState([]domain.RaisedAlert{{
		Instance:  domain.WindowInstance{Subscription: "zai", Bucket: "b", Window: "WEEKLY", ResetsAt: at.Add(time.Hour)},
		Threshold: 90, RaisedAt: at,
	}})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "alerts.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("alerts.json = %v", err)
	}
	loaded, err := store.Load()
	if err != nil || len(loaded.Raised()) != 1 || loaded.Raised()[0] != state.Raised()[0] {
		t.Fatalf("loaded = %+v, %v", loaded.Raised(), err)
	}
	withFailing := domain.NewAlertState(state.Raised(), domain.FailingSource{Subscription: "ollama", Failure: "BROKEN", Since: at})
	if err := store.Save(withFailing); err != nil {
		t.Fatal(err)
	}
	if loaded, err := store.Load(); err != nil || len(loaded.Failing()) != 1 || loaded.Failing()[0] != withFailing.Failing()[0] || len(loaded.Raised()) != 1 {
		t.Fatalf("failing sources = %+v, %v", loaded.Failing(), err)
	}
	// A state written before failing sources were kept still loads.
	if err := os.WriteFile(filepath.Join(dir, "alerts.json"), []byte(`{"version":1,"raised":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded, err := store.Load(); err != nil || len(loaded.Failing()) != 0 {
		t.Fatalf("older state = %+v, %v", loaded.Failing(), err)
	}
	for _, content := range []string{`{`, `{"version":2}`, `{"version":1,"raised":[{"threshold":0}]}`, `{"version":1,"raised":[],"failing":[{"subscription":"zai"}]}`} {
		if err := os.WriteFile(filepath.Join(dir, "alerts.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatalf("%s accepted", content)
		}
	}
}
