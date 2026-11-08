// Package offeringsfile keeps the model catalogue between collections in
// $XDG_STATE_HOME/ataecina/offerings.json (0600): which provider
// describes each subscription, when it was checked and refreshed, its validator and the
// models kept of each provider.
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

const (
	formatVersion = 1
	maxBytes      = 8 << 20
)

type document struct {
	Version     int                `json:"version"`
	Interests   []interestDocument `json:"interests"`
	CheckedAt   time.Time          `json:"checked_at"`
	RefreshedAt time.Time          `json:"refreshed_at"`
	Validator   string             `json:"validator"`
	Providers   []providerDocument `json:"providers"`
}

type interestDocument struct {
	Subscription string `json:"subscription"`
	Provider     string `json:"provider"`
}

type providerDocument struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Models []modelDocument `json:"models"`
}

type modelDocument struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	InputPrice  *float64 `json:"input_price"`
	OutputPrice *float64 `json:"output_price"`
	ReleasedOn  string   `json:"released_on,omitempty"`
}

// Store reads and writes offerings.json in the state directory.
type Store struct {
	dir string
}

// NewStore keeps the catalogue in dir.
func NewStore(dir string) Store { return Store{dir: dir} }

func (s Store) path() string { return filepath.Join(s.dir, "offerings.json") }

// Load reads the kept catalogue; a missing file is domain.ErrNoCatalogue.
func (s Store) Load() (domain.Catalogue, error) {
	data, err := privatefs.ReadFile(s.path(), maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Catalogue{}, domain.ErrNoCatalogue
	}
	if err != nil {
		return domain.Catalogue{}, err
	}
	var stored document
	if err := json.Unmarshal(data, &stored); err != nil {
		return domain.Catalogue{}, fmt.Errorf("%s: %w", s.path(), err)
	}
	if stored.Version != formatVersion {
		return domain.Catalogue{}, fmt.Errorf("%s: unknown version %d", s.path(), stored.Version)
	}
	catalogue := domain.Catalogue{
		CheckedAt: stored.CheckedAt, RefreshedAt: stored.RefreshedAt, Validator: stored.Validator,
	}
	for _, interest := range stored.Interests {
		catalogue.Interests = append(catalogue.Interests, domain.Interest(interest))
	}
	for _, provider := range stored.Providers {
		kept := domain.ProviderOfferings{Provider: provider.ID, Name: provider.Name}
		for _, model := range provider.Models {
			kept.Models = append(kept.Models, decodeModel(model))
		}
		catalogue.Providers = append(catalogue.Providers, kept)
	}
	return catalogue, nil
}

// Save replaces the kept catalogue atomically.
func (s Store) Save(catalogue domain.Catalogue) error {
	stored := document{
		Version: formatVersion, Interests: []interestDocument{}, CheckedAt: catalogue.CheckedAt.UTC(),
		RefreshedAt: catalogue.RefreshedAt.UTC(), Validator: catalogue.Validator, Providers: []providerDocument{},
	}
	for _, interest := range catalogue.Interests {
		stored.Interests = append(stored.Interests, interestDocument(interest))
	}
	for _, provider := range catalogue.Providers {
		kept := providerDocument{ID: provider.Provider, Name: provider.Name, Models: []modelDocument{}}
		for _, model := range provider.Models {
			kept.Models = append(kept.Models, encodeModel(model))
		}
		stored.Providers = append(stored.Providers, kept)
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

func encodeModel(model domain.Offering) modelDocument {
	stored := modelDocument{ID: model.Model, Name: model.Name}
	if model.Priced {
		input, output := model.InputPrice, model.OutputPrice
		stored.InputPrice, stored.OutputPrice = &input, &output
	}
	if !model.ReleasedOn.IsZero() {
		stored.ReleasedOn = model.ReleasedOn.Format(time.DateOnly)
	}
	return stored
}

func decodeModel(stored modelDocument) domain.Offering {
	model := domain.Offering{Model: stored.ID, Name: stored.Name}
	if stored.InputPrice != nil && stored.OutputPrice != nil {
		model.Priced, model.InputPrice, model.OutputPrice = true, *stored.InputPrice, *stored.OutputPrice
	}
	if day, err := time.Parse(time.DateOnly, stored.ReleasedOn); err == nil {
		model.ReleasedOn = day
	}
	return model
}
