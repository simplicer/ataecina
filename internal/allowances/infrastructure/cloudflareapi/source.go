// Package cloudflareapi reads the Workers AI daily allocation through
// Cloudflare's GraphQL Analytics API (documented; dataset fields to verify
// live): one fixed query, with the account id and today's UTC date as
// variables, sums aiInferenceAdaptiveGroups.sum.totalNeurons against the
// 10 000 neurons granted every day (SPECS §3.3). The token needs only
// Account Analytics: Read, so it cannot run inference.
package cloudflareapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.cloudflare.com"

const (
	graphqlPath = "/client/v4/graphql"
	// DailyNeurons is the allocation every Workers AI account gets per UTC day.
	DailyNeurons = 10_000
	query        = `query AtaecinaNeurons($accountTag: string!, $date: Date!) {
  viewer {
    accounts(filter: {accountTag: $accountTag}) {
      aiInferenceAdaptiveGroups(limit: 10000, filter: {date_geq: $date}) {
        sum { totalNeurons }
      }
    }
  }
}`
)

// ErrNoAccount reports an answer without the asked account: the token
// cannot read it.
var ErrNoAccount = errors.New("the account is not visible to this token")

// QueryError is an error the GraphQL API answered with HTTP 200.
type QueryError struct{ Message string }

func (e QueryError) Error() string { return "GraphQL error: " + e.Message }

// Source reads the Workers AI allocation of one account.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	account  string
	now      func() time.Time
}

// New reads account with the named API token through endpoint.
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, account string, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, account: account, now: now}
}

// Read sums today's neurons.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	today := now.UTC().Truncate(24 * time.Hour)
	document, err := Request(s.account, today)
	if err != nil {
		return domain.Reading{}, err
	}
	body, err := providerhttp.Post(ctx, s.endpoint, graphqlPath, key, document, now)
	if err != nil {
		return domain.Reading{}, err
	}
	neurons, err := DecodeNeurons(body)
	if err != nil {
		return domain.Reading{}, s.classify(err)
	}
	used, err := domain.NewUsedPercent(neurons / DailyNeurons * 100)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now,
		Meters: []domain.Meter{domain.NewMeter("", domain.DailyWindow(), used, today.Add(24*time.Hour))},
	})
}

// classify turns a refused query into KEY_REJECTED and anything else into
// CONTRACT_CHANGED.
func (s Source) classify(err error) error {
	var queryError QueryError
	refused := errors.Is(err, ErrNoAccount) ||
		(errors.As(err, &queryError) && strings.Contains(strings.ToLower(queryError.Message), "not authorized"))
	if refused {
		return domain.Failure(domain.ErrorKeyRejected,
			s.endpoint.Host()+" refused the analytics query: the token needs Account Analytics: Read on this account", nil)
	}
	return providerhttp.Unexpected(s.endpoint.Host(), err)
}

// Request is the fixed query with its variables.
func Request(account string, day time.Time) ([]byte, error) {
	return json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]string{"accountTag": account, "date": day.UTC().Format("2006-01-02")},
	})
}

// DecodeNeurons sums totalNeurons over the groups of the first account.
func DecodeNeurons(body []byte) (float64, error) {
	var document struct {
		Data *struct {
			Viewer *struct {
				Accounts []struct {
					Groups []struct {
						Sum *struct {
							TotalNeurons *float64 `json:"totalNeurons"`
						} `json:"sum"`
					} `json:"aiInferenceAdaptiveGroups"`
				} `json:"accounts"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, err
	}
	if len(document.Errors) > 0 {
		message := document.Errors[0].Message
		if len(message) > 120 {
			message = message[:120]
		}
		return 0, QueryError{Message: message}
	}
	if document.Data == nil || document.Data.Viewer == nil {
		return 0, errors.New("data.viewer is required")
	}
	if len(document.Data.Viewer.Accounts) == 0 {
		return 0, ErrNoAccount
	}
	total := 0.0
	for _, group := range document.Data.Viewer.Accounts[0].Groups {
		if group.Sum == nil || group.Sum.TotalNeurons == nil {
			return 0, errors.New("sum.totalNeurons is required in every group")
		}
		neurons := *group.Sum.TotalNeurons
		if neurons < 0 {
			return 0, fmt.Errorf("totalNeurons %v", neurons)
		}
		total += neurons
	}
	return total, nil
}
