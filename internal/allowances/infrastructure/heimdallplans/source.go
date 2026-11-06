// Package heimdallplans reads what Heimdall observes about the plans it
// serves (SPECS §3.6, ADR 0013): GET /v1/plans on its admin API, on
// loopback, with the owner's Heimdall admin token. A subscription names
// the provider whose plans it follows. Per Heimdall's OpenAPI contract a
// plan carries window_used_ratio (0–1), next_reset_at, available and
// unavailable_reason; versions that do not report the usage yet read as
// UNAVAILABLE with the next reset in the hint.
package heimdallplans

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

// SourceID is the source this package implements.
const SourceID domain.SourceID = "heimdall"

const plansPath = "/v1/plans"

// Plan is one plan as Heimdall's admin API describes it.
type Plan struct {
	ID       string
	Provider string
	// UsedRatio is nil when Heimdall does not report the window usage.
	UsedRatio *float64
	// NextResetAt is zero when unknown.
	NextResetAt time.Time
	// Unavailable is empty when the plan is available.
	Unavailable string
}

// Source reads the plans of one provider.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	now      func() time.Time
}

// New reads with the named admin token through endpoint (loopback).
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, now: now}
}

// Read asks Heimdall for its plans and keeps those of the subscription's provider.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	token, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer token.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, plansPath, token, now)
	if err != nil {
		return domain.Reading{}, err
	}
	plans, err := DecodePlans(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	var mine []Plan
	for _, plan := range plans {
		if plan.Provider == subscription.Provider().String() {
			mine = append(mine, plan)
		}
	}
	if len(mine) == 0 {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable,
			fmt.Sprintf("Heimdall serves no plan of provider %q", subscription.Provider()), nil)
	}
	meters, notes, err := observe(mine)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	if len(meters) == 0 {
		notes = append(notes, "this Heimdall reports no window usage yet")
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, strings.Join(notes, "; "), nil)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityObserved, Outcome: domain.OutcomeOK, ObservedAt: now,
		Meters: meters, Hint: strings.Join(notes, "; "),
	})
}

// observe turns plans into meters (one bucket per plan when there are
// several) and notes on unavailable plans and unknown usage.
func observe(plans []Plan) ([]domain.Meter, []string, error) {
	var meters []domain.Meter
	var notes []string
	for _, plan := range plans {
		bucket := domain.NewBucketID("")
		if len(plans) > 1 {
			bucket = domain.NewBucketID(plan.ID)
		}
		ratio := plan.UsedRatio
		if ratio == nil && plan.Unavailable == "quota_exhausted" {
			exhausted := 1.0
			ratio = &exhausted
		}
		if plan.Unavailable != "" {
			notes = append(notes, plan.ID+": "+strings.ReplaceAll(plan.Unavailable, "_", " "))
		}
		if ratio == nil {
			if !plan.NextResetAt.IsZero() {
				notes = append(notes, plan.ID+": next reset "+plan.NextResetAt.UTC().Format(time.RFC3339))
			}
			continue
		}
		used, err := domain.UsedPercentFromFraction(*ratio)
		if err != nil {
			return nil, nil, err
		}
		// Heimdall gives the window's next reset, not its length.
		meters = append(meters, domain.NewMeter(bucket, domain.BillingCycleWindow(), used, plan.NextResetAt))
	}
	return meters, notes, nil
}

// knownReasons are the unavailable_reason values of Heimdall's contract.
var knownReasons = map[string]bool{"peak_window": true, "quota_exhausted": true, "insufficient_balance": true, "unauthorized_account": true}

// DecodePlans reads the data of GET /v1/plans.
func DecodePlans(body []byte) ([]Plan, error) {
	var document struct {
		Data *[]struct {
			ID                *string  `json:"id"`
			Provider          *string  `json:"provider"`
			Available         *bool    `json:"available"`
			UnavailableReason *string  `json:"unavailable_reason"`
			NextResetAt       *string  `json:"next_reset_at"`
			WindowUsedRatio   *float64 `json:"window_used_ratio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.Data == nil {
		return nil, errors.New("data is required")
	}
	plans := make([]Plan, 0, len(*document.Data))
	for _, raw := range *document.Data {
		if raw.ID == nil || *raw.ID == "" || raw.Provider == nil || *raw.Provider == "" {
			return nil, errors.New("every plan needs an id and a provider")
		}
		plan := Plan{ID: *raw.ID, Provider: *raw.Provider, UsedRatio: raw.WindowUsedRatio}
		if plan.UsedRatio != nil && *plan.UsedRatio < 0 {
			return nil, errors.New("negative window_used_ratio")
		}
		if raw.NextResetAt != nil {
			at, err := time.Parse(time.RFC3339, *raw.NextResetAt)
			if err != nil {
				return nil, fmt.Errorf("next_reset_at: %w", err)
			}
			plan.NextResetAt = at.UTC()
		}
		if raw.Available != nil && !*raw.Available {
			plan.Unavailable = "unavailable"
			if raw.UnavailableReason != nil && knownReasons[*raw.UnavailableReason] {
				plan.Unavailable = *raw.UnavailableReason
			}
		}
		plans = append(plans, plan)
	}
	return plans, nil
}
