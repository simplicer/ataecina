package offeringsfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const offersMaxBytes = 1 << 20

type offersDocument struct {
	Format      int             `json:"format"`
	RefreshedAt time.Time       `json:"refreshed_at"`
	Offers      []offerDocument `json:"offers"`
}

type offerDocument struct {
	ID        string  `json:"id"`
	Provider  string  `json:"provider"`
	Plan      string  `json:"plan"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Period    string  `json:"period"`
	URL       string  `json:"url"`
	Sponsored bool    `json:"sponsored"`
	Ends      string  `json:"ends,omitempty"`
}

// OffersStore keeps the offers in $XDG_STATE_HOME/ataecina/offers.json
// (0600).
type OffersStore struct {
	dir string
}

// NewOffersStore keeps the offers in dir.
func NewOffersStore(dir string) OffersStore { return OffersStore{dir: dir} }

func (s OffersStore) path() string { return filepath.Join(s.dir, "offers.json") }

// Load reads the kept offers; a missing file is domain.ErrNoOffers.
func (s OffersStore) Load() (domain.Offers, error) {
	data, err := privatefs.ReadFile(s.path(), offersMaxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Offers{}, domain.ErrNoOffers
	}
	if err != nil {
		return domain.Offers{}, err
	}
	var stored offersDocument
	if err := json.Unmarshal(data, &stored); err != nil {
		return domain.Offers{}, fmt.Errorf("%s: %w", s.path(), err)
	}
	if stored.Format != formatVersion {
		return domain.Offers{}, fmt.Errorf("%s: unknown format %d", s.path(), stored.Format)
	}
	offers := domain.Offers{RefreshedAt: stored.RefreshedAt}
	for _, item := range stored.Offers {
		offer := domain.Offer{
			ID: item.ID, Provider: item.Provider, Plan: item.Plan, Title: item.Title, Price: item.Price,
			Currency: item.Currency, Period: item.Period, URL: item.URL, Sponsored: item.Sponsored,
		}
		if ends, err := time.Parse(time.DateOnly, item.Ends); err == nil {
			offer.Ends = ends
		}
		offers.Items = append(offers.Items, offer)
	}
	return offers, nil
}

// Save replaces the kept offers atomically.
func (s OffersStore) Save(offers domain.Offers) error {
	stored := offersDocument{Format: formatVersion, RefreshedAt: offers.RefreshedAt.UTC(), Offers: []offerDocument{}}
	for _, offer := range offers.Items {
		item := offerDocument{
			ID: offer.ID, Provider: offer.Provider, Plan: offer.Plan, Title: offer.Title, Price: offer.Price,
			Currency: offer.Currency, Period: offer.Period, URL: offer.URL, Sponsored: offer.Sponsored,
		}
		if !offer.Ends.IsZero() {
			item.Ends = offer.Ends.Format(time.DateOnly)
		}
		stored.Offers = append(stored.Offers, item)
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	return privatefs.WriteFile(s.path(), data)
}
