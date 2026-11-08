package modelsdev_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/modelsdev"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func model(provider domain.ProviderOfferings, id string) (domain.Offering, bool) {
	for _, offering := range provider.Models {
		if offering.Model == id {
			return offering, true
		}
	}
	return domain.Offering{}, false
}

func TestOnlyTheWantedProvidersAreKept(t *testing.T) {
	providers, err := modelsdev.DecodeCatalogue(read(t, "api.json"), []string{"openrouter", "zai-coding-plan", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 || providers[0].Provider != "openrouter" || providers[0].Name != "OpenRouter" || providers[1].Provider != "zai-coding-plan" {
		t.Fatalf("providers = %+v", providers)
	}
	openrouter := providers[0]
	if len(openrouter.Models) != 3 {
		t.Fatalf("openrouter kept %d models (the deprecated one must go)", len(openrouter.Models))
	}
	free, _ := model(openrouter, "example/coder-mini:free")
	if !free.Free() || !free.ReleasedOn.Equal(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("free model = %+v", free)
	}
	paid, _ := model(openrouter, "example/coder-max")
	if paid.Free() || paid.InputPrice != 1.25 || paid.OutputPrice != 5 || paid.ReleasedOn.Month() != time.March {
		t.Fatalf("paid model = %+v", paid)
	}
	router, _ := model(openrouter, "example/router")
	if router.Priced || router.Free() || router.Name != "bRouter/b" || !router.ReleasedOn.IsZero() {
		t.Fatalf("model without a price = %+v", router)
	}
}

func TestAChangedCatalogueIsRefused(t *testing.T) {
	for _, body := range []string{`[]`, `{}`, `{"p":{"models":[]}}`, `{"p":{"models":{"m":{"cost":{"input":2e6,"output":0}}}}}`} {
		if _, err := modelsdev.DecodeCatalogue([]byte(body), []string{"p"}); err == nil {
			t.Fatalf("DecodeCatalogue(%s) accepted", body)
		}
	}
	if _, err := modelsdev.DecodeCatalogue(read(t, "api.json"), []string{"elsewhere"}); err == nil {
		t.Fatal("a negative price was accepted")
	}
	if providers, err := modelsdev.DecodeCatalogue([]byte(`{"Bad Id":{"models":{}}}`), []string{"Bad Id"}); err != nil || len(providers) != 0 {
		t.Fatalf("a provider id that is not plain = %+v, %v", providers, err)
	}
}

func TestEveryTextIsPlainAndShort(t *testing.T) {
	long := strings.Repeat("é", 300)
	providers, err := modelsdev.DecodeCatalogue([]byte(`{"p":{"name":"","models":{"k":{"id":"","name":"`+long+`"}}}}`), []string{"p"})
	if err != nil {
		t.Fatal(err)
	}
	if providers[0].Name != "p" || providers[0].Models[0].Model != "k" || len([]rune(providers[0].Models[0].Name)) != 120 {
		t.Fatalf("texts = %+v", providers[0])
	}
}

// FuzzDecodeCatalogue checks that no catalogue panics the decoder or lets
// markup through.
func FuzzDecodeCatalogue(f *testing.F) {
	f.Add(read(f, "api.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		providers, err := modelsdev.DecodeCatalogue(body, []string{"openrouter", "zai-coding-plan", "p"})
		if err != nil {
			return
		}
		for _, provider := range providers {
			for _, offering := range provider.Models {
				if strings.ContainsAny(offering.Name+offering.Model+provider.Name, "<>&") {
					t.Fatalf("markup kept from %q", body)
				}
			}
		}
	})
}
