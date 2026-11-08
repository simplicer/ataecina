// Package openrouterbenchmarks reads the Artificial Analysis Intelligence
// Index as OpenRouter passes it on (GET
// https://openrouter.ai/api/v1/benchmarks?source=artificial-analysis, with
// the owner's OpenRouter API key as Bearer), for owners without an
// Artificial Analysis key of their own. Each row names an OpenRouter model
// (`model_permaslug`, dated) and its `display_name`, with
// `intelligence_index` (null when not measured).
package openrouterbenchmarks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Origin is the only host this adapter contacts.
const Origin = "https://openrouter.ai"

const benchmarksPath = "/api/v1/benchmarks?source=artificial-analysis"

// ErrKeyRejected is OpenRouter refusing the key.
var ErrKeyRejected = errors.New("OpenRouter rejected the key (HTTP 401 or 403)")

// Source fetches the ranking with the owner's OpenRouter key.
type Source struct {
	endpoint httpclient.Endpoint
	key      func(context.Context) (secret.Key, error)
}

// New asks endpoint with the key that key gives.
func New(endpoint httpclient.Endpoint, key func(context.Context) (secret.Key, error)) Source {
	return Source{endpoint: endpoint.Larger(4<<20, 20*time.Second), key: key}
}

// Fetch reads the benchmarks once.
func (s Source) Fetch(ctx context.Context) (domain.Ranking, error) {
	key, err := s.key(ctx)
	if err != nil {
		return domain.Ranking{}, err
	}
	defer key.Wipe()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+string(key.Reveal()))
	response, err := s.endpoint.Get(ctx, benchmarksPath, header)
	if err != nil {
		return domain.Ranking{}, fmt.Errorf("%s: %w", s.endpoint.Host(), err)
	}
	switch {
	case response.Status == http.StatusUnauthorized || response.Status == http.StatusForbidden:
		return domain.Ranking{}, ErrKeyRejected
	case response.Status != http.StatusOK:
		return domain.Ranking{}, fmt.Errorf("%s answered %d", s.endpoint.Host(), response.Status)
	}
	scores, err := Decode(response.Body)
	if err != nil {
		return domain.Ranking{}, fmt.Errorf("unexpected answer from %s: %w", s.endpoint.Host(), err)
	}
	return domain.Ranking{Via: "OpenRouter", Scores: scores}, nil
}

// Decode reads the Artificial Analysis rows with an intelligence index;
// each gives two scores, by display name and by OpenRouter slug, so that a
// model is found under either.
func Decode(body []byte) ([]domain.Score, error) {
	var document struct {
		Data *[]struct {
			Source       string   `json:"source"`
			Slug         string   `json:"model_permaslug"`
			Name         string   `json:"display_name"`
			Intelligence *float64 `json:"intelligence_index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.Data == nil {
		return nil, errors.New("data is required")
	}
	scores := []domain.Score{}
	for _, row := range *document.Data {
		if row.Source != "artificial-analysis" || row.Intelligence == nil {
			continue
		}
		index := *row.Intelligence
		if math.IsNaN(index) || index < 0 || index > 1000 {
			return nil, errors.New("an intelligence index is out of range")
		}
		for _, name := range []string{row.Name, row.Slug} {
			if plain := domain.PlainName(name); plain != "" {
				scores = append(scores, domain.Score{Name: plain, Index: index})
			}
		}
	}
	return scores, nil
}
