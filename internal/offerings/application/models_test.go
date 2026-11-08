package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/application"
	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

type rankingSource struct {
	answer domain.Ranking
	err    error
}

func (r rankingSource) Fetch(context.Context) (domain.Ranking, error) { return r.answer, r.err }

type rankingStore struct {
	kept    domain.Ranking
	loadErr error
	saveErr error
	saved   []domain.Ranking
}

func (r *rankingStore) Load() (domain.Ranking, error) { return r.kept, r.loadErr }

func (r *rankingStore) Save(ranking domain.Ranking) error {
	r.saved = append(r.saved, ranking)
	return r.saveErr
}

var ranking = domain.Ranking{Version: "5.0", Scores: []domain.Score{
	{Name: "GLM-5.3 (Max)", Index: 61},
	{Name: "GLM-5.3 (Low)", Index: 52},
	{Name: "Paid", Index: 70},
}}

func TestTheRankingIsKeptWhenAsked(t *testing.T) {
	store := &rankingStore{}
	got, err := application.NewRefreshRanking(rankingSource{answer: ranking}, store, clock{now}).Refresh(context.Background())
	if err != nil || !got.RefreshedAt.Equal(now) || len(store.saved) != 1 || store.saved[0].Version != "5.0" {
		t.Fatalf("Refresh = %+v, %v", got, err)
	}
	if _, err := application.NewRefreshRanking(rankingSource{err: errors.New("401")}, store, clock{now}).Refresh(context.Background()); err == nil || len(store.saved) != 1 {
		t.Fatalf("a failed fetch = %v, saved %d", err, len(store.saved))
	}
	broken := &rankingStore{saveErr: errors.New("disk full")}
	if _, err := application.NewRefreshRanking(rankingSource{answer: ranking}, broken, clock{now}).Refresh(context.Background()); err == nil {
		t.Fatal("a ranking that cannot be kept went unnoticed")
	}
}

func TestModelsAreListedBestFirst(t *testing.T) {
	glm := domain.Offering{Model: "z-ai/glm-5.3:free", Name: "GLM 5.3 (free)", Priced: true}
	catalogue := domain.Catalogue{
		RefreshedAt: now,
		Interests:   []domain.Interest{{Subscription: "openrouter", Provider: "openrouter"}, {Subscription: "gone", Provider: "gone"}},
		Providers: []domain.ProviderOfferings{{Provider: "openrouter", Name: "OpenRouter", Models: []domain.Offering{
			{Model: "b", Name: "B"}, {Model: "paid", Name: "Paid", Priced: true, InputPrice: 1}, glm, {Model: "a", Name: "A"},
		}}},
	}
	listing := application.NewListModels(&store{kept: catalogue}, &rankingStore{kept: ranking}).List()
	if len(listing.Subscriptions) != 1 || listing.RankingVersion != "5.0" || !listing.CatalogueRefreshedAt.Equal(now) {
		t.Fatalf("listing = %+v", listing)
	}
	models := listing.Subscriptions[0].Models
	order := []string{models[0].Model, models[1].Model, models[2].Model, models[3].Model}
	if order[0] != "paid" || order[1] != glm.Model || order[2] != "a" || order[3] != "b" {
		t.Fatalf("order = %v", order)
	}
	if !models[1].HasIndex || models[1].Index != 61 || models[2].HasIndex {
		t.Fatalf("indices = %+v", models)
	}
	unranked := application.NewListModels(&store{kept: catalogue}, &rankingStore{loadErr: domain.ErrNoRanking}).List()
	if unranked.Subscriptions[0].Models[0].HasIndex || unranked.RankingVersion != "" {
		t.Fatalf("without a ranking = %+v", unranked)
	}
	if empty := application.NewListModels(&store{loadErr: domain.ErrNoCatalogue}, &rankingStore{}).List(); len(empty.Subscriptions) != 0 {
		t.Fatalf("without a catalogue = %+v", empty)
	}
}

func TestUpdateAsksForTheCatalogueEvenWhenFresh(t *testing.T) {
	kept := &store{kept: domain.Catalogue{Interests: onOpenRouter, CheckedAt: now, RefreshedAt: now, Validator: `"v1"`, Providers: []domain.ProviderOfferings{openrouter}}}
	fetcher := &source{answer: domain.Fetched{NotModified: true}}
	asked, err := application.NewRefreshOfferingsNow(fetcher, kept, clock{now.Add(time.Minute)}).Refresh(context.Background(), onOpenRouter)
	if !asked || err != nil || fetcher.validators[0] != `"v1"` {
		t.Fatalf("Refresh now = %v, %v, %q", asked, err, fetcher.validators)
	}
}
