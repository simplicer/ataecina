//go:build integration

package offeringsfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/offeringsfile"
)

func TestTheRankingIsKeptPrivately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store := offeringsfile.NewRankingStore(dir)
	if _, err := store.Load(); !errors.Is(err, domain.ErrNoRanking) {
		t.Fatalf("Load before any save = %v", err)
	}
	at := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	if err := store.Save(domain.Ranking{Version: "5", Via: "OpenRouter", RefreshedAt: at, Scores: []domain.Score{{Name: "GLM-5.3 (Max)", Index: 61.2}}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "intelligence.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("intelligence.json = %v, %v", info, err)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Version != "5" || loaded.Via != "OpenRouter" || !loaded.RefreshedAt.Equal(at) || loaded.Scores[0].Index != 61.2 {
		t.Fatalf("loaded = %+v, %v", loaded, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "intelligence.json"), []byte(`{"format": 9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("an unknown format was read")
	}
}
