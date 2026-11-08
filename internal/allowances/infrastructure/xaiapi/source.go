// Package xaiapi reads the prepaid credit of an xAI (Grok API) team with a
// management key (documented: GET
// https://management-api.x.ai/v1/billing/teams/{team}/prepaid/balance).
// The total is a string in US cents; purchases are recorded as negative
// changes, so the credit left is the total's magnitude (sign convention to
// verify live).
package xaiapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strconv"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://management-api.x.ai"

const maxCents = 1e11

// Source reads one xAI team's prepaid credit.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	team     string
	now      func() time.Time
}

// New reads the team's credit with the named management key.
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, team string, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, team: team, now: now}
}

// Read asks for the team's prepaid balance.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, "/v1/billing/teams/"+url.PathEscape(s.team)+"/prepaid/balance", key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	cents, err := DecodeBalance(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	left, err := providerhttp.Dollars(math.Abs(cents) / 100)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	balance, _ := domain.NewBalance(&left, nil)
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Balance: &balance,
	})
}

// DecodeBalance reads total.val, an integer number of US cents as a
// string.
func DecodeBalance(body []byte) (float64, error) {
	var document struct {
		Total *struct {
			Val *string `json:"val"`
		} `json:"total"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, err
	}
	if document.Total == nil || document.Total.Val == nil {
		return 0, errors.New("total.val is required")
	}
	cents, err := strconv.ParseFloat(*document.Total.Val, 64)
	if err != nil || math.IsNaN(cents) || math.Abs(cents) > maxCents {
		return 0, errors.New("total.val is not an amount of cents")
	}
	return cents, nil
}
