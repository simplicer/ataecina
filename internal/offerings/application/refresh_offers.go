package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// RefreshOffers fetches the published coding plan offers again, only when
// the owner runs `ataecina update` (SPECS §12, ADR 0017).
type RefreshOffers struct {
	source domain.OffersSource
	lists  domain.OfferLists
	clock  domain.Clock
}

// NewRefreshOffers prepares one refresh.
func NewRefreshOffers(source domain.OffersSource, lists domain.OfferLists, clock domain.Clock) RefreshOffers {
	return RefreshOffers{source: source, lists: lists, clock: clock}
}

// Refresh keeps the offers fetched and says how many are current; a
// failure keeps the ones already kept.
func (r RefreshOffers) Refresh(ctx context.Context) (int, error) {
	items, err := r.source.Fetch(ctx)
	if err != nil {
		return 0, fmt.Errorf("refresh the offers: %w", err)
	}
	offers := domain.Offers{RefreshedAt: r.clock.Now(), Items: items}
	if err := r.lists.Save(offers); err != nil {
		return 0, fmt.Errorf("keep the offers: %w", err)
	}
	return len(offers.CurrentAt(offers.RefreshedAt)), nil
}
