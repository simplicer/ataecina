// Package zaiapi reads the Z.ai GLM Coding Plan windows with the owner's
// API key from the undocumented GET /api/monitor/usage/quota/limit (SPECS
// §3; the field meanings come from other clients and must be verified live).
package zaiapi

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
const Origin = "https://api.z.ai"

const quotaPath = "/api/monitor/usage/quota/limit"

// Units of a limit's window, as Z.ai numbers them.
const (
	unitHours = 3
	unitWeeks = 6
)

// errRejected marks an answer whose code says the key is not accepted.
var errRejected = errors.New("the key is not accepted")

// Plan is what the quota answer says about the subscription.
type Plan struct {
	Level  string
	Meters []domain.Meter
}

// Source reads one Z.ai subscription.
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

// Read asks for the quota windows.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, quotaPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	plan, err := DecodeQuota(body)
	if errors.Is(err, errRejected) {
		return domain.Reading{}, domain.Failure(domain.ErrorKeyRejected, s.endpoint.Host()+" rejected the key: set it again with `ataecina keys set`", nil)
	}
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: domain.NewPlan(plan.Level), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK,
		ObservedAt: now, Meters: plan.Meters,
	})
}

type limit struct {
	Type          string   `json:"type"`
	Unit          int      `json:"unit"`
	Number        int      `json:"number"`
	Percentage    *float64 `json:"percentage"`
	NextResetTime *int64   `json:"nextResetTime"`
}

// DecodeQuota reads {code, success, data: {limits: [...], level}}. Token or
// credit limits become meters: unit 3 is hours (5 → five hours), unit 6 is
// weeks; percentage is the share used; nextResetTime is epoch
// milliseconds and may be absent for an untouched window. Other limit
// types (tool calls) are not quota windows and are skipped.
func DecodeQuota(body []byte) (Plan, error) {
	var document struct {
		Code    int  `json:"code"`
		Success bool `json:"success"`
		Data    *struct {
			Limits []limit `json:"limits"`
			Level  string  `json:"level"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Plan{}, err
	}
	if document.Code == 401 || document.Code == 403 {
		return Plan{}, errRejected
	}
	if !document.Success || document.Data == nil {
		return Plan{}, errors.New("the answer is not a successful quota")
	}
	plan := Plan{Level: document.Data.Level}
	seen := map[domain.Window]bool{}
	for _, item := range document.Data.Limits {
		if item.Type != "TOKENS_LIMIT" && item.Type != "CREDIT_LIMIT" {
			continue
		}
		window, ok := windowOf(item.Unit, item.Number)
		if !ok || seen[window] {
			continue
		}
		if item.Percentage == nil {
			return Plan{}, errors.New("a limit without percentage")
		}
		used, err := domain.NewUsedPercent(*item.Percentage)
		if err != nil {
			return Plan{}, err
		}
		var resetsAt time.Time
		if item.NextResetTime != nil && *item.NextResetTime > 0 && *item.NextResetTime < math.MaxInt64/1000 {
			resetsAt = time.UnixMilli(*item.NextResetTime)
		}
		seen[window] = true
		plan.Meters = append(plan.Meters, domain.NewMeter("", window, used, resetsAt))
	}
	if len(plan.Meters) == 0 {
		return Plan{}, errors.New("no token or credit window in the answer")
	}
	return plan, nil
}

func windowOf(unit, number int) (domain.Window, bool) {
	if number <= 0 || number > 1000 {
		return domain.Window{}, false
	}
	var minutes int
	switch unit {
	case unitHours:
		minutes = number * 60
	case unitWeeks:
		minutes = number * 7 * 24 * 60
	default:
		return domain.Window{}, false
	}
	window, err := domain.WindowOfMinutes(minutes)
	return window, err == nil
}
