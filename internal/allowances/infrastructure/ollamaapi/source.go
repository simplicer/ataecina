// Package ollamaapi reads Ollama Cloud with the owner's API key
// (undocumented, called by the ollama.com usage page): GET /api/usage
// gives the session and weekly limits as fractions of their caps, without
// their reset instants (SPECS §3.3).
package ollamaapi

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
const Origin = "https://ollama.com"

const usagePath = "/api/usage"

// Limits are the fractions (0–1) of each cap already used; nil when absent.
type Limits struct {
	Session *float64
	Weekly  *float64
}

// Source reads one Ollama Cloud subscription.
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

// Read asks for the usage of the key's account.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, usagePath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	limits, err := DecodeUsage(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	var meters []domain.Meter
	for _, limit := range []struct {
		fraction *float64
		window   domain.Window
	}{
		{limits.Session, domain.FiveHourWindow()},
		{limits.Weekly, domain.WeeklyWindow()},
	} {
		if limit.fraction == nil {
			continue
		}
		used, err := domain.UsedPercentFromFraction(*limit.fraction)
		if err != nil {
			return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
		}
		// Ollama does not say when a window is reborn.
		meters = append(meters, domain.NewMeter("", limit.window, used, time.Time{}))
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Meters: meters,
	})
}

// DecodeUsage reads limits.session.usage and limits.weekly.usage; at least
// one is required.
func DecodeUsage(body []byte) (Limits, error) {
	type limit struct {
		Usage *float64 `json:"usage"`
	}
	var document struct {
		Limits *struct {
			Session *limit `json:"session"`
			Weekly  *limit `json:"weekly"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Limits{}, err
	}
	if document.Limits == nil {
		return Limits{}, errors.New("limits is required (monthly-credit plans are not read yet)")
	}
	var limits Limits
	if session := document.Limits.Session; session != nil {
		limits.Session = session.Usage
	}
	if weekly := document.Limits.Weekly; weekly != nil {
		limits.Weekly = weekly.Usage
	}
	if limits.Session == nil && limits.Weekly == nil {
		return Limits{}, errors.New("limits.session.usage or limits.weekly.usage is required")
	}
	for _, fraction := range []*float64{limits.Session, limits.Weekly} {
		if fraction != nil && *fraction < 0 {
			return Limits{}, errors.New("negative usage")
		}
	}
	return limits, nil
}
