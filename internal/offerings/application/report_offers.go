package application

import (
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// ReportOffers reads the kept offers for the displays; it never connects
// (ADR 0005).
type ReportOffers struct {
	lists domain.OfferLists
	clock domain.Clock
}

// NewReportOffers reads what `ataecina update` kept.
func NewReportOffers(lists domain.OfferLists, clock domain.Clock) ReportOffers {
	return ReportOffers{lists: lists, clock: clock}
}

// Report lists the offers that have not ended and when they were fetched;
// without any, nothing.
func (r ReportOffers) Report() ([]domain.Offer, time.Time) {
	offers, err := r.lists.Load()
	if err != nil {
		return nil, time.Time{}
	}
	return offers.CurrentAt(r.clock.Now()), offers.RefreshedAt
}
