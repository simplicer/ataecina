// Package codexlogs reads the rate limits that Codex writes into its own
// session logs (~/.codex/sessions/**/rollout-*.jsonl), incrementally and
// read-only (SPECS §3.2). Only `timestamp` and `payload.rate_limits` of
// `token_count` events are extracted; nothing else of a line is kept.
package codexlogs

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
)

var rateLimitsMarker = []byte(`"rate_limits"`)

// errMalformed reports a token_count event whose rate limits Ataecina
// cannot read: the log format may have changed.
var errMalformed = errors.New("malformed rate limits")

type logLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type       string          `json:"type"`
		RateLimits json.RawMessage `json:"rate_limits"`
	} `json:"payload"`
}

type loggedLimits struct {
	LimitID   *string        `json:"limit_id"`
	PlanType  *string        `json:"plan_type"`
	Primary   *loggedWindow  `json:"primary"`
	Secondary *loggedWindow  `json:"secondary"`
	Credits   *loggedCredits `json:"credits"`
}

type loggedWindow struct {
	UsedPercent     *float64 `json:"used_percent"`
	WindowMinutes   *int     `json:"window_minutes"`
	ResetsAt        *int64   `json:"resets_at"`
	ResetsInSeconds *int64   `json:"resets_in_seconds"`
}

type loggedCredits struct {
	HasCredits bool    `json:"has_credits"`
	Unlimited  bool    `json:"unlimited"`
	Balance    *string `json:"balance"`
}

// DecodeLine extracts the rate limits of one log line. ok is false for
// every line that is not a token_count event carrying rate limits; err is
// errMalformed when such an event cannot be read.
func DecodeLine(line []byte) (limits codexlimits.Limits, ok bool, err error) {
	if !bytes.Contains(line, rateLimitsMarker) {
		return codexlimits.Limits{}, false, nil
	}
	var event logLine
	if json.Unmarshal(line, &event) != nil || event.Type != "event_msg" || event.Payload.Type != "token_count" {
		return codexlimits.Limits{}, false, nil
	}
	if len(event.Payload.RateLimits) == 0 || string(event.Payload.RateLimits) == "null" {
		return codexlimits.Limits{}, false, nil
	}
	observedAt, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		return codexlimits.Limits{}, false, errMalformed
	}
	var logged loggedLimits
	if err := json.Unmarshal(event.Payload.RateLimits, &logged); err != nil {
		return codexlimits.Limits{}, false, errMalformed
	}
	limits = codexlimits.Limits{ObservedAt: observedAt.UTC(), LimitID: codexlimits.MainLimit}
	if logged.LimitID != nil && *logged.LimitID != "" {
		limits.LimitID = *logged.LimitID
	}
	if logged.PlanType != nil {
		limits.Plan = *logged.PlanType
	}
	for _, w := range []*loggedWindow{logged.Primary, logged.Secondary} {
		if w == nil {
			continue
		}
		window, err := decodeWindow(*w, limits.ObservedAt)
		if err != nil {
			return codexlimits.Limits{}, false, err
		}
		limits.Windows = append(limits.Windows, window)
	}
	if c := logged.Credits; c != nil {
		limits.Credits = &codexlimits.Credits{HasCredits: c.HasCredits, Unlimited: c.Unlimited}
		if c.Balance != nil {
			limits.Credits.Balance = *c.Balance
		}
	}
	return limits, true, nil
}

func decodeWindow(w loggedWindow, observedAt time.Time) (codexlimits.Window, error) {
	if w.UsedPercent == nil || w.WindowMinutes == nil {
		return codexlimits.Window{}, errMalformed
	}
	var resetsAt time.Time
	switch {
	case w.ResetsAt != nil && *w.ResetsAt > 0:
		resetsAt = time.Unix(*w.ResetsAt, 0)
	case w.ResetsInSeconds != nil && *w.ResetsInSeconds >= 0:
		// Older Codex versions logged the reset relative to the event.
		resetsAt = observedAt.Add(time.Duration(*w.ResetsInSeconds) * time.Second)
	}
	window, err := codexlimits.NewWindow(*w.WindowMinutes, *w.UsedPercent, resetsAt)
	if err != nil {
		return codexlimits.Window{}, errMalformed
	}
	return window, nil
}
