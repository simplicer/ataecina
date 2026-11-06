// Package orcarouterapi reads Orca Router with the owner's API key through
// its two documented billing summaries, which follow the OpenAI-compatible
// shape (SPECS §3.3): GET /v1/dashboard/billing/subscription for the
// allowance (hard_limit_usd, the key's total) and its expiry
// (access_until, Unix seconds, 0 when none), and GET
// /v1/dashboard/billing/usage for what was spent (total_usage, in US
// cents). Names and units are to verify live.
package orcarouterapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.orcarouter.ai"

const (
	subscriptionPath = "/v1/dashboard/billing/subscription"
	usagePath        = "/v1/dashboard/billing/usage"
	// unlimitedUSD and above is how OpenAI-compatible gateways say "no limit".
	unlimitedUSD = 1_000_000
)

// Allowance is what /v1/dashboard/billing/subscription says.
type Allowance struct {
	HardLimitUSD float64
	// ExpiresAt is zero when the allowance does not expire.
	ExpiresAt time.Time
}

// Source reads one Orca Router subscription.
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

// Read asks for the allowance and what was spent of it.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, subscriptionPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	allowance, err := DecodeSubscription(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	body, err = providerhttp.Fetch(ctx, s.endpoint, usagePath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	spentUSD, err := DecodeUsage(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	balance, hint, err := balanceOf(allowance, spentUSD)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now,
		Balance: &balance, Hint: hint,
	})
}

func balanceOf(allowance Allowance, spentUSD float64) (domain.Balance, string, error) {
	spent, err := providerhttp.Dollars(spentUSD)
	if err != nil {
		return domain.Balance{}, "", err
	}
	if allowance.HardLimitUSD >= unlimitedUSD {
		balance, err := domain.NewBalance(nil, &spent)
		return balance, "the key has no spending limit", err
	}
	remaining, err := providerhttp.Dollars(math.Max(0, allowance.HardLimitUSD-spentUSD))
	if err != nil {
		return domain.Balance{}, "", err
	}
	balance, err := domain.NewBalance(&remaining, &spent)
	hint := ""
	if !allowance.ExpiresAt.IsZero() {
		hint = "the allowance expires on " + allowance.ExpiresAt.UTC().Format("2006-01-02")
	}
	return balance, hint, err
}

// DecodeSubscription reads hard_limit_usd and access_until.
func DecodeSubscription(body []byte) (Allowance, error) {
	var document struct {
		HardLimitUSD *float64 `json:"hard_limit_usd"`
		AccessUntil  *int64   `json:"access_until"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Allowance{}, err
	}
	if document.HardLimitUSD == nil || *document.HardLimitUSD < 0 {
		return Allowance{}, errors.New("hard_limit_usd is required and not negative")
	}
	allowance := Allowance{HardLimitUSD: *document.HardLimitUSD}
	if until := document.AccessUntil; until != nil && *until > 0 {
		if *until > 1<<40 {
			return Allowance{}, errors.New("access_until is not in Unix seconds")
		}
		allowance.ExpiresAt = time.Unix(*until, 0).UTC()
	}
	return allowance, nil
}

// DecodeUsage reads total_usage (US cents) and returns US dollars.
func DecodeUsage(body []byte) (float64, error) {
	var document struct {
		TotalUsage *float64 `json:"total_usage"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, err
	}
	if document.TotalUsage == nil || *document.TotalUsage < 0 {
		return 0, errors.New("total_usage is required and not negative")
	}
	return *document.TotalUsage / 100, nil
}
