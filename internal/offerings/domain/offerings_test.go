package domain_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

var now = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)

func interests(providers ...string) []domain.Interest {
	var list []domain.Interest
	for i, provider := range providers {
		list = append(list, domain.Interest{Subscription: provider + string(rune('a'+i)), Provider: provider})
	}
	return list
}

func TestWhenTheCatalogueIsDue(t *testing.T) {
	wanted := interests("vercel", "openrouter")
	fresh := domain.Catalogue{Interests: interests("openrouter", "vercel"), CheckedAt: now.Add(-time.Hour), RefreshedAt: now.Add(-time.Hour)}
	for name, c := range map[string]struct {
		catalogue domain.Catalogue
		at        time.Time
		asked     []domain.Interest
		due       bool
	}{
		"never fetched":           {domain.Catalogue{}, now, wanted, true},
		"fresh, in another order": {fresh, now, wanted, false},
		"a day old":               {fresh, now.Add(23 * time.Hour), wanted, true},
		"other providers":         {domain.Catalogue{Interests: interests("openrouter"), CheckedAt: now, RefreshedAt: now}, now, wanted, true},
		"failed half an hour ago": {domain.Catalogue{Interests: wanted, CheckedAt: now.Add(-30 * time.Minute)}, now, wanted, false},
		"failed an hour ago":      {domain.Catalogue{Interests: wanted, CheckedAt: now.Add(-time.Hour)}, now, wanted, true},
		"one more subscription":   {fresh, now, interests("vercel", "openrouter", "vercel"), false},
	} {
		if got := c.catalogue.DueAt(c.at, c.asked); got != c.due {
			t.Fatalf("%s: DueAt = %v", name, got)
		}
	}
}

func TestTheValidatorOnlyConfirmsTheSameProviders(t *testing.T) {
	catalogue := domain.Catalogue{Interests: interests("openrouter"), CheckedAt: now, RefreshedAt: now, Validator: `"v1"`}
	if catalogue.ValidatorFor(interests("openrouter")) != `"v1"` || catalogue.ValidatorFor(interests("vercel")) != "" {
		t.Fatal("validator offered for other providers")
	}
	if (domain.Catalogue{Validator: `"v1"`}).ValidatorFor(nil) != "" {
		t.Fatal("validator of a catalogue never refreshed")
	}
	unchanged := catalogue.Refreshed(domain.Fetched{NotModified: true}, interests("openrouter"), now.Add(time.Hour))
	if unchanged.Validator != `"v1"` {
		t.Fatalf("an unchanged answer without a validator lost it: %+v", unchanged)
	}
}

func TestFreeAndNewModels(t *testing.T) {
	future := domain.Offering{Model: "soon", Priced: true, ReleasedOn: now.Add(48 * time.Hour)}
	old := domain.Offering{Model: "old", Priced: true, ReleasedOn: now.AddDate(0, -2, 0)}
	recent := domain.Offering{Model: "recent", Priced: true, InputPrice: 0.1, ReleasedOn: now.AddDate(0, 0, -10)}
	if future.NewAt(now) || old.NewAt(now) || !recent.NewAt(now) || recent.Free() {
		t.Fatal("new or free misjudged")
	}
	if (domain.Offering{Priced: true, OutputPrice: 1}).Free() || (domain.Offering{}).Free() {
		t.Fatal("a model priced for output, or without a price, is not free")
	}
	catalogue := domain.Catalogue{
		Interests: []domain.Interest{{Subscription: "s", Provider: "p"}, {Subscription: "t", Provider: "q"}},
		Providers: []domain.ProviderOfferings{{Provider: "p", Models: []domain.Offering{future, old, recent}}},
	}
	summaries := catalogue.Summaries(now)
	if len(summaries) != 1 || summaries[0].Subscription != "s" {
		t.Fatalf("summaries = %+v", summaries)
	}
	if summary := summaries[0]; summary.Models != 3 || len(summary.Free) != 2 || len(summary.New) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}
