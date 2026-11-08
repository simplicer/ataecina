package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/application"
	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

var now = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)

type clock struct{ at time.Time }

func (c clock) Now() time.Time { return c.at }

type store struct {
	kept    domain.Catalogue
	loadErr error
	saveErr error
	saved   []domain.Catalogue
}

func (s *store) Load() (domain.Catalogue, error) { return s.kept, s.loadErr }

func (s *store) Save(catalogue domain.Catalogue) error {
	s.saved = append(s.saved, catalogue)
	return s.saveErr
}

type source struct {
	answer     domain.Fetched
	err        error
	providers  [][]string
	validators []string
}

func (s *source) Fetch(_ context.Context, providers []string, validator string) (domain.Fetched, error) {
	s.providers = append(s.providers, providers)
	s.validators = append(s.validators, validator)
	return s.answer, s.err
}

var (
	openrouter = domain.ProviderOfferings{Provider: "openrouter", Name: "OpenRouter", Models: []domain.Offering{
		{Model: "free-b", Name: "B (free)", Priced: true, ReleasedOn: now.AddDate(0, 0, -3)},
		{Model: "paid", Name: "Paid", Priced: true, InputPrice: 1, OutputPrice: 3, ReleasedOn: now.AddDate(-1, 0, 0)},
		{Model: "free-a", Name: "A (free)", Priced: true},
	}}
	onOpenRouter = []domain.Interest{{Subscription: "openrouter", Provider: "openrouter"}}
)

func refresh(t *testing.T, fetcher *source, kept *store, at time.Time, interests []domain.Interest) (bool, error) {
	t.Helper()
	return application.NewRefreshOfferings(fetcher, kept, clock{at}).Refresh(context.Background(), interests)
}

func TestAFirstRefreshKeepsTheProvidersOfTheSubscriptions(t *testing.T) {
	kept, fetcher := &store{loadErr: domain.ErrNoCatalogue}, &source{answer: domain.Fetched{Validator: `"v1"`, Providers: []domain.ProviderOfferings{openrouter}}}
	interests := []domain.Interest{{Subscription: "zen", Provider: "opencode"}, {Subscription: "openrouter", Provider: "openrouter"}, {Subscription: "zen-2", Provider: "opencode"}}
	asked, err := refresh(t, fetcher, kept, now, interests)
	if err != nil || !asked || len(kept.saved) != 1 {
		t.Fatalf("Refresh = %v, %v, saved %d", asked, err, len(kept.saved))
	}
	saved := kept.saved[0]
	if saved.Validator != `"v1"` || !saved.RefreshedAt.Equal(now) || len(saved.Providers) != 1 || fetcher.validators[0] != "" {
		t.Fatalf("saved = %+v, validators %q", saved, fetcher.validators)
	}
	if asked := fetcher.providers[0]; len(asked) != 2 || asked[0] != "opencode" || asked[1] != "openrouter" {
		t.Fatalf("asked for %q", asked)
	}
	if saved.Interests[0].Subscription != "openrouter" {
		t.Fatalf("interests = %+v", saved.Interests)
	}
}

func TestAFreshCatalogueIsNotAskedAgain(t *testing.T) {
	kept := &store{kept: domain.Catalogue{Interests: onOpenRouter, CheckedAt: now.Add(-time.Hour), RefreshedAt: now.Add(-time.Hour), Providers: []domain.ProviderOfferings{openrouter}}}
	fetcher := &source{}
	if asked, err := refresh(t, fetcher, kept, now, onOpenRouter); asked || err != nil || len(fetcher.validators) != 0 || len(kept.saved) != 0 {
		t.Fatalf("Refresh = %v, %v", asked, err)
	}
	if asked, _ := refresh(t, fetcher, kept, now, nil); asked {
		t.Fatal("asked for no subscription")
	}
	renamed := []domain.Interest{{Subscription: "router", Provider: "openrouter"}}
	if asked, err := refresh(t, fetcher, kept, now, renamed); asked || err != nil || len(kept.saved) != 1 || kept.saved[0].Interests[0].Subscription != "router" {
		t.Fatalf("a renamed subscription = %v, %v, %+v", asked, err, kept.saved)
	}
}

func TestADayOldCatalogueIsConfirmedWithItsValidator(t *testing.T) {
	dayOld := now.Add(-25 * time.Hour)
	kept := &store{kept: domain.Catalogue{Interests: onOpenRouter, CheckedAt: dayOld, RefreshedAt: dayOld, Validator: `"v1"`, Providers: []domain.ProviderOfferings{openrouter}}}
	fetcher := &source{answer: domain.Fetched{NotModified: true}}
	if _, err := refresh(t, fetcher, kept, now, onOpenRouter); err != nil {
		t.Fatal(err)
	}
	saved := kept.saved[0]
	if fetcher.validators[0] != `"v1"` || saved.Validator != `"v1"` || len(saved.Providers) != 1 || !saved.RefreshedAt.Equal(now) {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestAnotherProviderAsksForTheWholeCatalogue(t *testing.T) {
	kept := &store{kept: domain.Catalogue{Interests: onOpenRouter, CheckedAt: now, RefreshedAt: now, Validator: `"v1"`}}
	fetcher := &source{answer: domain.Fetched{Validator: `"v2"`}}
	more := append([]domain.Interest{{Subscription: "vercel", Provider: "vercel"}}, onOpenRouter...)
	if asked, err := refresh(t, fetcher, kept, now, more); !asked || err != nil {
		t.Fatalf("Refresh = %v, %v", asked, err)
	}
	if fetcher.validators[0] != "" || len(kept.saved[0].Interests) != 2 {
		t.Fatalf("validator %q, saved %+v", fetcher.validators[0], kept.saved[0])
	}
}

func TestAFailedRefreshKeepsWhatWasKnownAndWaitsAnHour(t *testing.T) {
	dayOld := now.Add(-25 * time.Hour)
	kept := &store{kept: domain.Catalogue{Interests: onOpenRouter, CheckedAt: dayOld, RefreshedAt: dayOld, Providers: []domain.ProviderOfferings{openrouter}}}
	fetcher := &source{err: errors.New("offline")}
	if _, err := refresh(t, fetcher, kept, now, onOpenRouter); err == nil {
		t.Fatal("the failure was not returned")
	}
	failed := kept.saved[0]
	if !failed.CheckedAt.Equal(now) || !failed.RefreshedAt.Equal(dayOld) || len(failed.Providers) != 1 {
		t.Fatalf("failed = %+v", failed)
	}
	kept.kept = failed
	if asked, _ := refresh(t, fetcher, kept, now.Add(30*time.Minute), onOpenRouter); asked {
		t.Fatal("asked again within the hour")
	}
	if asked, _ := refresh(t, fetcher, kept, now.Add(time.Hour), onOpenRouter); !asked {
		t.Fatal("not asked again after an hour")
	}
}

func TestAnUnkeptCatalogueIsReported(t *testing.T) {
	kept := &store{loadErr: errors.New("corrupt"), saveErr: errors.New("disk full")}
	fetcher := &source{answer: domain.Fetched{Providers: []domain.ProviderOfferings{openrouter}}}
	if _, err := refresh(t, fetcher, kept, now, onOpenRouter); err == nil {
		t.Fatal("a catalogue that cannot be kept went unnoticed")
	}
}

func TestTheDisplaysGetTheFreeAndNewModels(t *testing.T) {
	interests := append([]domain.Interest{{Subscription: "vercel", Provider: "vercel"}}, onOpenRouter...)
	kept := &store{kept: domain.Catalogue{Interests: interests, RefreshedAt: now, Providers: []domain.ProviderOfferings{openrouter}}}
	summaries, refreshedAt := application.NewReportOfferings(kept, clock{now}).Report()
	if len(summaries) != 1 || !refreshedAt.Equal(now) {
		t.Fatalf("Report = %+v, %s", summaries, refreshedAt)
	}
	summary := summaries[0]
	if summary.Subscription != "openrouter" || summary.Models != 3 || len(summary.Free) != 2 || summary.Free[0].Model != "free-a" || len(summary.New) != 1 || summary.New[0].Model != "free-b" {
		t.Fatalf("summary = %+v", summary)
	}
	if none, at := application.NewReportOfferings(&store{loadErr: domain.ErrNoCatalogue}, clock{now}).Report(); none != nil || !at.IsZero() {
		t.Fatalf("no catalogue = %+v", none)
	}
}
