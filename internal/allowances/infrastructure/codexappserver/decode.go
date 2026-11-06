package codexappserver

import (
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
)

// errUnexpectedResult reports an answer whose shape Ataecina does not know.
var errUnexpectedResult = errors.New("unexpected account/rateLimits/read result")

type rateLimitsResult struct {
	RateLimits            *snapshot           `json:"rateLimits"`
	RateLimitsByLimitID   map[string]snapshot `json:"rateLimitsByLimitId"`
	RateLimitResetCredits *struct {
		AvailableCount *int `json:"availableCount"`
	} `json:"rateLimitResetCredits"`
}

type snapshot struct {
	LimitID   *string  `json:"limitId"`
	PlanType  *string  `json:"planType"`
	Primary   *window  `json:"primary"`
	Secondary *window  `json:"secondary"`
	Credits   *credits `json:"credits"`
}

type window struct {
	UsedPercent        *float64 `json:"usedPercent"`
	WindowDurationMins *int     `json:"windowDurationMins"`
	ResetsAt           *int64   `json:"resetsAt"`
}

type credits struct {
	HasCredits bool    `json:"hasCredits"`
	Unlimited  bool    `json:"unlimited"`
	Balance    *string `json:"balance"`
}

// DecodeRateLimits reads the result of account/rateLimits/read: the
// latest limits per limit id (rateLimitsByLimitId, or rateLimits when the
// map is absent) and the banked resets, nil when not reported.
func DecodeRateLimits(result []byte, observedAt time.Time) (map[string]codexlimits.Limits, *int, error) {
	var decoded rateLimitsResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, nil, errUnexpectedResult
	}
	byID := decoded.RateLimitsByLimitID
	if len(byID) == 0 && decoded.RateLimits != nil {
		byID = map[string]snapshot{codexlimits.MainLimit: *decoded.RateLimits}
	}
	if len(byID) == 0 {
		return nil, nil, errUnexpectedResult
	}
	latest := make(map[string]codexlimits.Limits, len(byID))
	for key, snap := range byID {
		limits, err := limitsOf(key, snap, observedAt)
		if err != nil {
			return nil, nil, err
		}
		latest[limits.LimitID] = limits
	}
	var banked *int
	if decoded.RateLimitResetCredits != nil && decoded.RateLimitResetCredits.AvailableCount != nil {
		if *decoded.RateLimitResetCredits.AvailableCount < 0 {
			return nil, nil, errUnexpectedResult
		}
		banked = decoded.RateLimitResetCredits.AvailableCount
	}
	return latest, banked, nil
}

func limitsOf(key string, snap snapshot, observedAt time.Time) (codexlimits.Limits, error) {
	limits := codexlimits.Limits{ObservedAt: observedAt.UTC(), LimitID: key}
	if snap.LimitID != nil && *snap.LimitID != "" {
		limits.LimitID = *snap.LimitID
	}
	if snap.PlanType != nil {
		limits.Plan = *snap.PlanType
	}
	for _, w := range []*window{snap.Primary, snap.Secondary} {
		if w == nil {
			continue
		}
		if w.UsedPercent == nil || w.WindowDurationMins == nil {
			return codexlimits.Limits{}, errUnexpectedResult
		}
		var resetsAt time.Time
		if w.ResetsAt != nil && *w.ResetsAt > 0 {
			resetsAt = time.Unix(*w.ResetsAt, 0)
		}
		decoded, err := codexlimits.NewWindow(*w.WindowDurationMins, *w.UsedPercent, resetsAt)
		if err != nil {
			return codexlimits.Limits{}, errUnexpectedResult
		}
		limits.Windows = append(limits.Windows, decoded)
	}
	if c := snap.Credits; c != nil {
		limits.Credits = &codexlimits.Credits{HasCredits: c.HasCredits, Unlimited: c.Unlimited}
		if c.Balance != nil {
			limits.Credits.Balance = *c.Balance
		}
	}
	return limits, nil
}
