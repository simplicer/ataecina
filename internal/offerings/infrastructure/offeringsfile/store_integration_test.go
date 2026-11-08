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

func TestTheCatalogueIsKeptPrivately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store := offeringsfile.NewStore(dir)
	if _, err := store.Load(); !errors.Is(err, domain.ErrNoCatalogue) {
		t.Fatalf("Load before any save = %v", err)
	}
	at := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	catalogue := domain.Catalogue{
		Interests: []domain.Interest{{Subscription: "router", Provider: "openrouter"}}, CheckedAt: at, RefreshedAt: at, Validator: `W/"v1"`,
		Providers: []domain.ProviderOfferings{{Provider: "openrouter", Name: "OpenRouter", Models: []domain.Offering{
			{Model: "free", Name: "Free", Priced: true, ReleasedOn: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)},
			{Model: "paid", Name: "Paid", Priced: true, InputPrice: 1.25, OutputPrice: 5},
			{Model: "unknown", Name: "Unknown"},
		}}},
	}
	if err := store.Save(catalogue); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "offerings.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("offerings.json = %v, %v", info, err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	models := loaded.Providers[0].Models
	if !loaded.RefreshedAt.Equal(at) || loaded.Validator != `W/"v1"` || len(models) != 3 || loaded.Interests[0].Subscription != "router" {
		t.Fatalf("loaded = %+v", loaded)
	}
	if !models[0].Free() || !models[0].ReleasedOn.Equal(catalogue.Providers[0].Models[0].ReleasedOn) || models[1].OutputPrice != 5 || models[2].Priced {
		t.Fatalf("models = %+v", models)
	}
}

func TestAStrangeFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	for _, content := range []string{"not json", `{"version": 2}`} {
		if err := os.WriteFile(filepath.Join(dir, "offerings.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := offeringsfile.NewStore(dir).Load(); err == nil || errors.Is(err, domain.ErrNoCatalogue) {
			t.Fatalf("%q: Load = %v", content, err)
		}
	}
}
