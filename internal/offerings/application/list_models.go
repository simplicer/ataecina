package application

import (
	"cmp"
	"slices"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// RankedOffering is a model with its intelligence index, when the ranking
// knows it.
type RankedOffering struct {
	domain.Offering
	Index    float64
	HasIndex bool
}

// SubscriptionModels are the models of one subscription's provider, the
// best ranked first, then by name.
type SubscriptionModels struct {
	Subscription string
	Provider     string
	Name         string
	Models       []RankedOffering
}

// Listing is every subscription's models and how fresh the data is.
type Listing struct {
	Subscriptions        []SubscriptionModels
	CatalogueRefreshedAt time.Time
	RankingVersion       string
	RankingVia           string
	RankingRefreshedAt   time.Time
}

// ListModels reads the kept catalogue and ranking for the displays; it
// never asks a source (ADR 0005).
type ListModels struct {
	catalogues domain.Catalogues
	rankings   domain.Rankings
}

// NewListModels reads what the collector and `ataecina update` kept.
func NewListModels(catalogues domain.Catalogues, rankings domain.Rankings) ListModels {
	return ListModels{catalogues: catalogues, rankings: rankings}
}

// List gives the models of every subscription the catalogue describes;
// without a catalogue it lists nothing, without a ranking no index.
func (l ListModels) List() Listing {
	catalogue, err := l.catalogues.Load()
	if err != nil {
		return Listing{}
	}
	ranking, err := l.rankings.Load()
	if err != nil {
		ranking = domain.Ranking{}
	}
	indexer := ranking.Indexer()
	listing := Listing{
		CatalogueRefreshedAt: catalogue.RefreshedAt, RankingVersion: ranking.Version, RankingVia: ranking.Via,
		RankingRefreshedAt: ranking.RefreshedAt,
	}
	for _, interest := range catalogue.Interests {
		offerings, ok := catalogue.Provider(interest.Provider)
		if !ok {
			continue
		}
		models := make([]RankedOffering, 0, len(offerings.Models))
		for _, offering := range offerings.Models {
			index, known := indexer.IndexOf(offering)
			models = append(models, RankedOffering{Offering: offering, Index: index, HasIndex: known})
		}
		slices.SortStableFunc(models, byIndexThenName)
		listing.Subscriptions = append(listing.Subscriptions, SubscriptionModels{
			Subscription: interest.Subscription, Provider: offerings.Provider, Name: offerings.Name, Models: models,
		})
	}
	return listing
}

func byIndexThenName(a, b RankedOffering) int {
	switch {
	case a.HasIndex && !b.HasIndex:
		return -1
	case !a.HasIndex && b.HasIndex:
		return 1
	case a.HasIndex && a.Index != b.Index:
		return cmp.Compare(b.Index, a.Index)
	}
	return cmp.Compare(a.Name, b.Name)
}
