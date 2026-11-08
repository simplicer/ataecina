// Package application holds the model catalogue use cases, one per file.
package application

import (
	"context"
	"fmt"
	"slices"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// RefreshOfferings asks the catalogue's source again when the kept
// catalogue is due (once a day, an hour after a failure, or when other
// providers are wanted) and keeps what it answers (SPECS §11).
type RefreshOfferings struct {
	source     domain.OfferingsSource
	catalogues domain.Catalogues
	clock      domain.Clock
}

// NewRefreshOfferings prepares the refresh of one collection.
func NewRefreshOfferings(source domain.OfferingsSource, catalogues domain.Catalogues, clock domain.Clock) RefreshOfferings {
	return RefreshOfferings{source: source, catalogues: catalogues, clock: clock}
}

// Refresh checks the catalogue for the subscriptions' providers and
// reports whether it asked the source. A catalogue that is not due keeps
// its offerings for the new interests. A failed check keeps the offerings
// already known; its error is returned for the log. An unreadable kept
// catalogue is replaced.
func (r RefreshOfferings) Refresh(ctx context.Context, interests []domain.Interest) (bool, error) {
	if len(interests) == 0 {
		return false, nil
	}
	kept, err := r.catalogues.Load()
	if err != nil {
		kept = domain.Catalogue{}
	}
	now := r.clock.Now()
	if !kept.DueAt(now, interests) {
		if next := kept.For(interests); !slices.Equal(next.Interests, kept.Interests) {
			return false, r.keep(next)
		}
		return false, nil
	}
	fetched, fetchErr := r.source.Fetch(ctx, domain.ProvidersOf(interests), kept.ValidatorFor(interests))
	next := kept.Failed(now)
	if fetchErr == nil {
		next = kept.Refreshed(fetched, interests, now)
	}
	if err := r.keep(next); err != nil {
		return true, err
	}
	if fetchErr != nil {
		return true, fmt.Errorf("refresh the model catalogue: %w", fetchErr)
	}
	return true, nil
}

func (r RefreshOfferings) keep(catalogue domain.Catalogue) error {
	if err := r.catalogues.Save(catalogue); err != nil {
		return fmt.Errorf("keep the model catalogue: %w", err)
	}
	return nil
}
