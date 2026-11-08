// Package modelsdev reads the public model catalogue of models.dev (SPECS
// §11, ADR 0015): `GET https://models.dev/api.json`, without any key,
// revalidated with its ETag. The answer maps each provider id to its name
// and its models; Ataecina keeps only the providers it is asked for, drops
// deprecated models and keeps each model's id, name, list price (USD per
// million tokens) and release day.
package modelsdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://models.dev"

const (
	catalogPath = "/api.json"
	// MaxBody is the largest catalogue accepted after decompression; it
	// was 6 MB in November 2026.
	MaxBody = 24 << 20
	// Timeout bounds the whole download.
	Timeout = 25 * time.Second

	maxText   = 120
	maxPrice  = 1e6
	maxModels = 5000
)

var providerID = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)

// Source fetches the catalogue.
type Source struct {
	endpoint httpclient.Endpoint
}

// New fetches through endpoint, allowed the catalogue's size and time.
func New(endpoint httpclient.Endpoint) Source {
	return Source{endpoint: endpoint.Larger(MaxBody, Timeout)}
}

// Fetch asks for the catalogue once; with a validator, an unchanged
// catalogue is answered as not modified.
func (s Source) Fetch(ctx context.Context, providers []string, validator string) (domain.Fetched, error) {
	header := http.Header{}
	if validator != "" {
		header.Set("If-None-Match", validator)
	}
	response, err := s.endpoint.Get(ctx, catalogPath, header)
	if err != nil {
		return domain.Fetched{}, fmt.Errorf("%s: %w", s.endpoint.Host(), err)
	}
	etag := response.Header.Get("ETag")
	switch response.Status {
	case http.StatusNotModified:
		return domain.Fetched{NotModified: true, Validator: etag}, nil
	case http.StatusOK:
	default:
		return domain.Fetched{}, fmt.Errorf("%s answered %d", s.endpoint.Host(), response.Status)
	}
	offerings, err := DecodeCatalogue(response.Body, providers)
	if err != nil {
		return domain.Fetched{}, fmt.Errorf("unexpected catalogue from %s: %w", s.endpoint.Host(), err)
	}
	return domain.Fetched{Validator: etag, Providers: offerings}, nil
}

type modelDocument struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	ReleaseDate string `json:"release_date"`
	Cost        *struct {
		Input  *float64 `json:"input"`
		Output *float64 `json:"output"`
	} `json:"cost"`
}

// DecodeCatalogue reads the wanted providers of the catalogue; a wanted
// provider the catalogue does not list is left out. Only the models of the
// wanted providers are decoded.
func DecodeCatalogue(body []byte, wanted []string) ([]domain.ProviderOfferings, error) {
	var document map[string]struct {
		Name   string          `json:"name"`
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if len(document) == 0 {
		return nil, errors.New("no provider")
	}
	var offerings []domain.ProviderOfferings
	for _, id := range wanted {
		provider, ok := document[id]
		if !ok || !providerID.MatchString(id) {
			continue
		}
		var models map[string]modelDocument
		if err := json.Unmarshal(provider.Models, &models); err != nil {
			return nil, fmt.Errorf("provider %s: %w", id, err)
		}
		if len(models) > maxModels {
			return nil, fmt.Errorf("provider %s lists %d models", id, len(models))
		}
		kept := domain.ProviderOfferings{Provider: id, Name: plain(provider.Name, id)}
		for key, model := range models {
			offering, keep, err := decodeModel(key, model)
			if err != nil {
				return nil, fmt.Errorf("provider %s: %w", id, err)
			}
			if keep {
				kept.Models = append(kept.Models, offering)
			}
		}
		offerings = append(offerings, kept)
	}
	return offerings, nil
}

// decodeModel keeps a model that is not deprecated; a negative or absurd
// price is a changed catalogue.
func decodeModel(key string, model modelDocument) (domain.Offering, bool, error) {
	if model.Status == "deprecated" {
		return domain.Offering{}, false, nil
	}
	id := plain(model.ID, key)
	offering := domain.Offering{Model: id, Name: plain(model.Name, id), ReleasedOn: releaseDay(model.ReleaseDate)}
	if model.Cost != nil && model.Cost.Input != nil && model.Cost.Output != nil {
		input, output := *model.Cost.Input, *model.Cost.Output
		if input < 0 || output < 0 || input > maxPrice || output > maxPrice {
			return domain.Offering{}, false, fmt.Errorf("model %s has an impossible price", id)
		}
		offering.Priced, offering.InputPrice, offering.OutputPrice = true, input, output
	}
	return offering, true, nil
}

// plain is a catalogue text made safe to show as plain text, or the
// fallback made safe when nothing is left of it.
func plain(text, fallback string) string {
	if cleaned := sanitized(text); cleaned != "" {
		return cleaned
	}
	return sanitized(fallback)
}

// sanitized drops control characters and markup characters and keeps at
// most 120 characters.
func sanitized(text string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' || r == '&' {
			return -1
		}
		return r
	}, strings.TrimSpace(text))
	if runes := []rune(cleaned); len(runes) > maxText {
		cleaned = string(runes[:maxText])
	}
	return cleaned
}

// releaseDay reads "2026-04-27" or "2026-04"; anything else is unknown.
func releaseDay(text string) time.Time {
	for _, layout := range []string{"2006-01-02", "2006-01"} {
		if day, err := time.Parse(layout, text); err == nil {
			return day
		}
	}
	return time.Time{}
}
