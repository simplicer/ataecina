// Package artificialanalysis reads the Artificial Analysis Intelligence
// Index of language models through its free Data API (documented: GET
// https://artificialanalysis.ai/api/v2/language/models/free, header
// x-api-key, 100 requests a day on the free tier, attribution required).
// The answer is paged (200 models a page); each model gives its name and
// evaluations.artificial_analysis_intelligence_index (null when not
// measured). The key is the owner's, sealed like a provider key.
package artificialanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Origin is the only host this adapter contacts.
const Origin = "https://artificialanalysis.ai"

const (
	modelsPath = "/api/v2/language/models/free"
	// maxPages bounds the pages asked for in one update.
	maxPages = 10
	maxBody  = 4 << 20
	timeout  = 20 * time.Second
)

// ErrKeyRejected is the API refusing the key.
var ErrKeyRejected = errors.New("Artificial Analysis rejected the key (HTTP 401 or 403): set it again with `ataecina keys set artificial-analysis`")

// Source fetches the ranking with the owner's key.
type Source struct {
	endpoint httpclient.Endpoint
	key      func(context.Context) (secret.Key, error)
}

// New asks endpoint with the key that key gives.
func New(endpoint httpclient.Endpoint, key func(context.Context) (secret.Key, error)) Source {
	return Source{endpoint: endpoint.Larger(maxBody, timeout), key: key}
}

// Fetch reads every page.
func (s Source) Fetch(ctx context.Context) (domain.Ranking, error) {
	key, err := s.key(ctx)
	if err != nil {
		return domain.Ranking{}, err
	}
	defer key.Wipe()
	var ranking domain.Ranking
	for page := 1; page <= maxPages; page++ {
		header := http.Header{}
		header.Set("x-api-key", string(key.Reveal()))
		response, err := s.endpoint.Get(ctx, modelsPath+"?page="+strconv.Itoa(page), header)
		if err != nil {
			return domain.Ranking{}, fmt.Errorf("%s: %w", s.endpoint.Host(), err)
		}
		switch {
		case response.Status == http.StatusUnauthorized || response.Status == http.StatusForbidden:
			return domain.Ranking{}, ErrKeyRejected
		case response.Status != http.StatusOK:
			return domain.Ranking{}, fmt.Errorf("%s answered %d", s.endpoint.Host(), response.Status)
		}
		version, scores, more, err := DecodePage(response.Body)
		if err != nil {
			return domain.Ranking{}, fmt.Errorf("unexpected answer from %s: %w", s.endpoint.Host(), err)
		}
		ranking.Version = version
		ranking.Scores = append(ranking.Scores, scores...)
		if !more {
			break
		}
	}
	return ranking, nil
}

// DecodePage reads intelligence_index_version, pagination.has_more and,
// for each model with an index, its name and
// evaluations.artificial_analysis_intelligence_index.
func DecodePage(body []byte) (string, []domain.Score, bool, error) {
	var document struct {
		Version    *float64 `json:"intelligence_index_version"`
		Pagination *struct {
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
		Data *[]struct {
			Name        string `json:"name"`
			Evaluations *struct {
				Index *float64 `json:"artificial_analysis_intelligence_index"`
			} `json:"evaluations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return "", nil, false, err
	}
	if document.Data == nil {
		return "", nil, false, errors.New("data is required")
	}
	version := ""
	if document.Version != nil {
		version = strconv.FormatFloat(*document.Version, 'f', -1, 64)
	}
	var scores []domain.Score
	for _, model := range *document.Data {
		if model.Evaluations == nil || model.Evaluations.Index == nil {
			continue
		}
		index := *model.Evaluations.Index
		if math.IsNaN(index) || index < 0 || index > 1000 {
			return "", nil, false, errors.New("an intelligence index is out of range")
		}
		if name := domain.PlainName(model.Name); name != "" {
			scores = append(scores, domain.Score{Name: name, Index: index})
		}
	}
	more := document.Pagination != nil && document.Pagination.HasMore
	return version, scores, more, nil
}
