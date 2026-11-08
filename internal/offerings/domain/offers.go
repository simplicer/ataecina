package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNoOffers is a list of offers that was never fetched.
var ErrNoOffers = errors.New("no offers yet")

// Offer is a coding plan on offer, published by the maintainer: who sells
// it, the plan, a short title, its price for a period, where to buy it,
// whether the link is sponsored (an affiliate link) and the last day it
// holds (zero when open-ended).
type Offer struct {
	ID        string
	Provider  string
	Plan      string
	Title     string
	Price     float64
	Currency  string
	Period    string
	URL       string
	Sponsored bool
	Ends      time.Time
}

// CurrentAt reports an offer that has not ended.
func (o Offer) CurrentAt(now time.Time) bool {
	return o.Ends.IsZero() || !now.After(o.Ends.Add(24*time.Hour))
}

// Offers are the offers fetched last and when.
type Offers struct {
	RefreshedAt time.Time
	Items       []Offer
}

// CurrentAt lists the offers that have not ended, in published order.
func (o Offers) CurrentAt(now time.Time) []Offer {
	var current []Offer
	for _, offer := range o.Items {
		if offer.CurrentAt(now) {
			current = append(current, offer)
		}
	}
	return current
}

// OffersSource fetches the published offers; none published is an empty
// list, not an error.
type OffersSource interface {
	Fetch(ctx context.Context) ([]Offer, error)
}

// OfferLists keeps the offers between updates.
type OfferLists interface {
	Load() (Offers, error)
	Save(Offers) error
}
