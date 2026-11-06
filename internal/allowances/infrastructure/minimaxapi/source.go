// Package minimaxapi reads the MiniMax Token Plan with the owner's plan
// key from the undocumented GET /v1/token_plan/remains (SPECS §3). MiniMax
// confirmed that its "*_usage_count" fields hold the REMAINING count; the
// explicit "*_remaining_percent" fields win when present. To verify live.
package minimaxapi

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
const Origin = "https://api.minimax.io"

const remainsPath = "/v1/token_plan/remains"

// Status codes of base_resp that mean the key is not accepted.
var rejectedCodes = map[int]bool{1004: true, 2049: true}

// errRejected marks an answer whose status says the key is not accepted.
var errRejected = errors.New("the key is not accepted")

// Source reads one MiniMax subscription.
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

// Read asks for what remains of every model's windows.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, remainsPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	meters, err := DecodeRemains(body)
	switch {
	case errors.Is(err, errRejected):
		return domain.Reading{}, domain.Failure(domain.ErrorKeyRejected, s.endpoint.Host()+" rejected the key (a Token Plan key is needed)", nil)
	case err != nil:
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	case len(meters) == 0:
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "MiniMax reported no quota for this key", nil)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Meters: meters,
	})
}

type modelRemains struct {
	ModelName                string   `json:"model_name"`
	StartTime                int64    `json:"start_time"`
	EndTime                  int64    `json:"end_time"`
	IntervalTotal            float64  `json:"current_interval_total_count"`
	IntervalCount            float64  `json:"current_interval_usage_count"`
	IntervalRemainingPercent *float64 `json:"current_interval_remaining_percent"`
	WeeklyTotal              float64  `json:"current_weekly_total_count"`
	WeeklyCount              float64  `json:"current_weekly_usage_count"`
	WeeklyRemainingPercent   *float64 `json:"current_weekly_remaining_percent"`
	WeeklyEndTime            int64    `json:"weekly_end_time"`
}

// DecodeRemains reads {model_remains: [...], base_resp: {status_code}}:
// per model, the interval window (its length is end − start, in epoch
// milliseconds) and the weekly window, each as the share used.
func DecodeRemains(body []byte) ([]domain.Meter, error) {
	var document struct {
		ModelRemains []modelRemains `json:"model_remains"`
		BaseResp     *struct {
			StatusCode int `json:"status_code"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.BaseResp == nil {
		return nil, errors.New("base_resp is required")
	}
	if rejectedCodes[document.BaseResp.StatusCode] {
		return nil, errRejected
	}
	if document.BaseResp.StatusCode != 0 {
		return nil, errors.New("base_resp reports a failure")
	}
	var meters []domain.Meter
	for _, model := range document.ModelRemains {
		bucket := domain.NewBucketID(model.ModelName)
		if model.EndTime > model.StartTime && model.StartTime > 0 {
			if used, ok, err := usedOf(model.IntervalRemainingPercent, model.IntervalTotal, model.IntervalCount); err != nil {
				return nil, err
			} else if ok {
				window, err := domain.WindowOfMinutes(int((model.EndTime - model.StartTime) / 60000))
				if err != nil {
					return nil, err
				}
				meters = append(meters, domain.NewMeter(bucket, window, used, millis(model.EndTime)))
			}
		}
		if used, ok, err := usedOf(model.WeeklyRemainingPercent, model.WeeklyTotal, model.WeeklyCount); err != nil {
			return nil, err
		} else if ok {
			meters = append(meters, domain.NewMeter(bucket, domain.WeeklyWindow(), used, millis(model.WeeklyEndTime)))
		}
	}
	return meters, nil
}

// usedOf prefers the explicit remaining percentage, else total and the
// remaining count; ok is false when the window has no quota.
func usedOf(remainingPercent *float64, total, remaining float64) (domain.UsedPercent, bool, error) {
	switch {
	case remainingPercent != nil:
		used, err := domain.NewUsedPercent(100 - math.Min(100, *remainingPercent))
		return used, err == nil, err
	case total > 0:
		used, err := domain.NewUsedPercent(math.Max(0, (total-remaining)/total*100))
		return used, err == nil, err
	}
	return 0, false, nil
}

func millis(value int64) time.Time {
	if value <= 0 || value > math.MaxInt64/1000 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}
