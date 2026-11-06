// Package openrouterapi reads OpenRouter with the owner's API key
// (documented): GET /api/v1/key for the key's usage, limit and free-model
// requests, and GET /api/v1/credits for the account balance when the key
// may read it.
package openrouterapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://openrouter.ai"

const (
	keyPath         = "/api/v1/key"
	creditsPath     = "/api/v1/credits"
	freeModelBucket = "free_models"
)

// KeyFacts are the fields of /api/v1/key Ataecina reads.
type KeyFacts struct {
	Usage          float64
	Limit          *float64
	LimitRemaining *float64
	FreeUsed       float64
	FreeLimit      float64
}

// Credits are the fields of /api/v1/credits Ataecina reads.
type Credits struct {
	Total float64
	Used  float64
}

// Source reads one OpenRouter subscription.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	now      func() time.Time
}

// New reads with the named key through endpoint.
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, now: now}
}

// Read asks for the key and, when allowed, the account credits.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, keyPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	facts, err := DecodeKey(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	var credits *Credits
	if body, err := providerhttp.Fetch(ctx, s.endpoint, creditsPath, key, now); err == nil {
		if decoded, err := DecodeCredits(body); err == nil {
			credits = &decoded
		}
	}
	reading := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now,
	}
	balance, complete, err := balanceOf(facts, credits)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	reading.Balance = &balance
	if !complete {
		reading.Outcome = domain.OutcomePartial
		reading.Hint = "the account balance needs a key that may read /api/v1/credits"
	}
	if facts.FreeLimit > 0 {
		used, err := domain.NewUsedPercent(facts.FreeUsed / facts.FreeLimit * 100)
		if err != nil {
			return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
		}
		midnight := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		reading.Meters = []domain.Meter{domain.NewMeter(freeModelBucket, domain.DailyWindow(), used, midnight)}
	}
	return domain.NewReading(reading)
}

// balanceOf prefers the account credits, then the key's own limit.
func balanceOf(facts KeyFacts, credits *Credits) (domain.Balance, bool, error) {
	spentValue, remainingValue, complete := facts.Usage, 0.0, false
	switch {
	case credits != nil:
		spentValue, remainingValue, complete = credits.Used, credits.Total-credits.Used, true
	case facts.Limit != nil && facts.LimitRemaining != nil:
		remainingValue, complete = *facts.LimitRemaining, true
	}
	spent, err := providerhttp.Dollars(spentValue)
	if err != nil {
		return domain.Balance{}, false, err
	}
	if !complete {
		balance, err := domain.NewBalance(nil, &spent)
		return balance, false, err
	}
	remaining, err := providerhttp.Dollars(remainingValue)
	if err != nil {
		return domain.Balance{}, false, err
	}
	balance, err := domain.NewBalance(&remaining, &spent)
	return balance, true, err
}

// DecodeKey reads the data of /api/v1/key.
func DecodeKey(body []byte) (KeyFacts, error) {
	var document struct {
		Data *struct {
			Usage                  *float64 `json:"usage"`
			Limit                  *float64 `json:"limit"`
			LimitRemaining         *float64 `json:"limit_remaining"`
			FreeModelDailyRequests *struct {
				Used  float64 `json:"used"`
				Limit float64 `json:"limit"`
			} `json:"free_model_daily_requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return KeyFacts{}, err
	}
	if document.Data == nil || document.Data.Usage == nil {
		return KeyFacts{}, errors.New("data.usage is required")
	}
	facts := KeyFacts{Usage: *document.Data.Usage, Limit: document.Data.Limit, LimitRemaining: document.Data.LimitRemaining}
	if free := document.Data.FreeModelDailyRequests; free != nil {
		if free.Used < 0 || free.Limit < 0 {
			return KeyFacts{}, errors.New("negative free-model requests")
		}
		facts.FreeUsed, facts.FreeLimit = free.Used, free.Limit
	}
	return facts, nil
}

// DecodeCredits reads the data of /api/v1/credits.
func DecodeCredits(body []byte) (Credits, error) {
	var document struct {
		Data *struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   *float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Credits{}, err
	}
	if document.Data == nil || document.Data.TotalCredits == nil || document.Data.TotalUsage == nil {
		return Credits{}, errors.New("data.total_credits and data.total_usage are required")
	}
	return Credits{Total: *document.Data.TotalCredits, Used: *document.Data.TotalUsage}, nil
}
