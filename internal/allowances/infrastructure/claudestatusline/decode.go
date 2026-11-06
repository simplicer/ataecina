// Package claudestatusline reads the rate limits Claude Code passes to its
// status line command (SPECS §3.1), keeps them as a signal file in
// $XDG_RUNTIME_DIR/ataecina/signals/ and serves that signal to the
// collector. Only `rate_limits` is extracted from the status line input.
package claudestatusline

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// ErrMalformed reports rate limits Ataecina cannot read.
var ErrMalformed = errors.New("malformed Claude rate limits")

// spendLimitBucket names the meter of a Claude apps gateway spend limit.
const spendLimitBucket = "spend_limit"

// Plausible reset instants (Unix seconds), to reject milliseconds or junk.
var (
	earliestReset = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	latestReset   = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
)

type statusLine struct {
	RateLimits *struct {
		FiveHour   *limit `json:"five_hour"`
		SevenDay   *limit `json:"seven_day"`
		SpendLimit *limit `json:"spend_limit"`
	} `json:"rate_limits"`
}

type limit struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

// DecodeRateLimits returns the meters of the status line input; none when
// Claude Code passed no rate limits (before the first API response, or for
// plans without them).
func DecodeRateLimits(input []byte) ([]domain.Meter, error) {
	var line statusLine
	if err := json.Unmarshal(input, &line); err != nil {
		return nil, ErrMalformed
	}
	if line.RateLimits == nil {
		return nil, nil
	}
	windows := []struct {
		limit  *limit
		bucket domain.BucketID
		window domain.Window
	}{
		{line.RateLimits.FiveHour, "", domain.FiveHourWindow()},
		{line.RateLimits.SevenDay, "", domain.WeeklyWindow()},
		{line.RateLimits.SpendLimit, spendLimitBucket, domain.BillingCycleWindow()},
	}
	var meters []domain.Meter
	for _, w := range windows {
		if w.limit == nil {
			continue
		}
		meter, err := decodeLimit(*w.limit, w.bucket, w.window)
		if err != nil {
			return nil, err
		}
		meters = append(meters, meter)
	}
	return meters, nil
}

func decodeLimit(l limit, bucket domain.BucketID, window domain.Window) (domain.Meter, error) {
	if l.UsedPercentage == nil {
		return domain.Meter{}, ErrMalformed
	}
	used, err := domain.NewUsedPercent(*l.UsedPercentage)
	if err != nil {
		return domain.Meter{}, ErrMalformed
	}
	var resetsAt time.Time
	if l.ResetsAt != nil {
		seconds := *l.ResetsAt
		if math.IsNaN(seconds) || seconds < float64(earliestReset) || seconds > float64(latestReset) {
			return domain.Meter{}, ErrMalformed
		}
		resetsAt = time.Unix(int64(seconds), 0)
	}
	return domain.NewMeter(bucket, window, used, resetsAt), nil
}
