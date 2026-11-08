package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/application"
	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

type offersSource struct {
	offers []domain.Offer
	err    error
}

func (o offersSource) Fetch(context.Context) ([]domain.Offer, error) { return o.offers, o.err }

type offerLists struct {
	kept    domain.Offers
	loadErr error
	saveErr error
	saved   []domain.Offers
}

func (o *offerLists) Load() (domain.Offers, error) { return o.kept, o.loadErr }

func (o *offerLists) Save(offers domain.Offers) error {
	o.saved = append(o.saved, offers)
	return o.saveErr
}

var offers = []domain.Offer{
	{ID: "open", Provider: "Z.ai", Plan: "Lite"},
	{ID: "ended", Provider: "MiniMax", Plan: "Plus", Ends: now.AddDate(0, 0, -2)},
	{ID: "today", Provider: "Kimi", Plan: "Pro", Ends: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)},
}

func TestOffersAreKeptWhenAsked(t *testing.T) {
	lists := &offerLists{}
	current, err := application.NewRefreshOffers(offersSource{offers: offers}, lists, clock{now}).Refresh(context.Background())
	if err != nil || current != 2 || len(lists.saved) != 1 || len(lists.saved[0].Items) != 3 || !lists.saved[0].RefreshedAt.Equal(now) {
		t.Fatalf("Refresh = %d, %v, %+v", current, err, lists.saved)
	}
	if _, err := application.NewRefreshOffers(offersSource{err: errors.New("offline")}, lists, clock{now}).Refresh(context.Background()); err == nil || len(lists.saved) != 1 {
		t.Fatal("a failed fetch replaced the kept offers")
	}
	broken := &offerLists{saveErr: errors.New("disk full")}
	if _, err := application.NewRefreshOffers(offersSource{offers: offers}, broken, clock{now}).Refresh(context.Background()); err == nil {
		t.Fatal("offers that cannot be kept went unnoticed")
	}
}

func TestOnlyCurrentOffersAreShown(t *testing.T) {
	shown, at := application.NewReportOffers(&offerLists{kept: domain.Offers{RefreshedAt: now, Items: offers}}, clock{now}).Report()
	if len(shown) != 2 || shown[0].ID != "open" || shown[1].ID != "today" || !at.Equal(now) {
		t.Fatalf("Report = %+v, %s", shown, at)
	}
	if none, at := application.NewReportOffers(&offerLists{loadErr: domain.ErrNoOffers}, clock{now}).Report(); none != nil || !at.IsZero() {
		t.Fatalf("without offers = %+v", none)
	}
}
