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

func TestTheOffersAreKeptPrivately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store := offeringsfile.NewOffersStore(dir)
	if _, err := store.Load(); !errors.Is(err, domain.ErrNoOffers) {
		t.Fatalf("Load before any save = %v", err)
	}
	at := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	kept := domain.Offers{RefreshedAt: at, Items: []domain.Offer{
		{ID: "a", Provider: "Example", Plan: "Lite", Price: 5, Currency: "USD", Period: "month", URL: "https://example.com", Sponsored: true, Ends: ends},
		{ID: "b", Provider: "Example", Plan: "Pro", URL: "https://example.com/pro"},
	}}
	if err := store.Save(kept); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "offers.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("offers.json = %v, %v", info, err)
	}
	loaded, err := store.Load()
	if err != nil || !loaded.RefreshedAt.Equal(at) || len(loaded.Items) != 2 || !loaded.Items[0].Ends.Equal(ends) || !loaded.Items[0].Sponsored || !loaded.Items[1].Ends.IsZero() {
		t.Fatalf("loaded = %+v, %v", loaded, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "offers.json"), []byte(`{"format": 7}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("an unknown format was read")
	}
}
