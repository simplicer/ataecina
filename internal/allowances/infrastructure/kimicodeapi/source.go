// Package kimicodeapi reads the Kimi Code membership limits with an API key
// from the Kimi Code Console (undocumented; the call the official kimi-cli
// makes for `/usage`): GET https://api.kimi.com/coding/v1/usages. The
// answer has a top-level `usage` (the weekly limit) and `limits[]`, each a
// window (`duration` in a `timeUnit`) with its limit, what remains and
// when it resets; numbers may come as strings. The OAuth token of
// `kimi login` is never used (ADR 0003).
package kimicodeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.kimi.com"

const usagesPath = "/coding/v1/usages"

// Limit is one window of the membership.
type Limit struct {
	Window   domain.Window
	Used     float64
	ResetsAt time.Time
}

// Usages is the membership's level and its windows.
type Usages struct {
	Level  string
	Limits []Limit
}

// Source reads one Kimi Code membership.
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

// Read asks for the membership's limits.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, usagesPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	usages, err := DecodeUsages(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	meters := make([]domain.Meter, 0, len(usages.Limits))
	for _, limit := range usages.Limits {
		used, err := domain.NewUsedPercent(limit.Used)
		if err != nil {
			return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
		}
		meters = append(meters, domain.NewMeter("", limit.Window, used, limit.ResetsAt))
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: domain.NewPlan(usages.Level), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK,
		ObservedAt: now, Meters: meters,
	})
}

// number accepts 100, "100" and "100.5".
type number float64

func (n *number) UnmarshalJSON(data []byte) error {
	text := strings.Trim(string(data), `"`)
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("not a number: %s", data)
	}
	*n = number(value)
	return nil
}

type detailDocument struct {
	Limit     *number `json:"limit"`
	Remaining *number `json:"remaining"`
	Used      *number `json:"used"`
	ResetTime *string `json:"resetTime"`
}

// DecodeUsages reads `usage` as the weekly window and each `limits[]`
// entry by its length; a window already read is not read twice. At least
// one window is required.
func DecodeUsages(body []byte) (Usages, error) {
	var document struct {
		Usage  *detailDocument `json:"usage"`
		Limits []struct {
			Window *struct {
				Duration *int   `json:"duration"`
				TimeUnit string `json:"timeUnit"`
			} `json:"window"`
			Detail *detailDocument `json:"detail"`
		} `json:"limits"`
		User *struct {
			Membership *struct {
				Level string `json:"level"`
			} `json:"membership"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Usages{}, err
	}
	var usages Usages
	if document.User != nil && document.User.Membership != nil {
		usages.Level = strings.TrimPrefix(document.User.Membership.Level, "LEVEL_")
	}
	add := func(window domain.Window, detail *detailDocument) error {
		for _, known := range usages.Limits {
			if known.Window == window {
				return nil
			}
		}
		limit, err := decodeDetail(window, detail)
		if err != nil {
			return err
		}
		usages.Limits = append(usages.Limits, limit)
		return nil
	}
	for _, entry := range document.Limits {
		if entry.Window == nil || entry.Window.Duration == nil {
			return Usages{}, errors.New("a limit needs window.duration")
		}
		window, err := windowOf(*entry.Window.Duration, entry.Window.TimeUnit)
		if err != nil {
			return Usages{}, err
		}
		if err := add(window, entry.Detail); err != nil {
			return Usages{}, err
		}
	}
	if document.Usage != nil {
		if err := add(domain.WeeklyWindow(), document.Usage); err != nil {
			return Usages{}, err
		}
	}
	if len(usages.Limits) == 0 {
		return Usages{}, errors.New("no usage and no limit")
	}
	return usages, nil
}

func windowOf(duration int, unit string) (domain.Window, error) {
	minutes := map[string]int{"TIME_UNIT_MINUTE": 1, "TIME_UNIT_HOUR": 60, "TIME_UNIT_DAY": 1440}[unit]
	if minutes == 0 || duration <= 0 || duration > 1_000_000 {
		return domain.Window{}, fmt.Errorf("a window of %d %q", duration, unit)
	}
	return domain.WindowOfMinutes(duration * minutes)
}

// decodeDetail reads limit and remaining (or used); used is a percentage
// of the limit.
func decodeDetail(window domain.Window, detail *detailDocument) (Limit, error) {
	if detail == nil || detail.Limit == nil || *detail.Limit <= 0 {
		return Limit{}, errors.New("a limit above zero is required")
	}
	limit := float64(*detail.Limit)
	var used float64
	switch {
	case detail.Used != nil:
		used = float64(*detail.Used)
	case detail.Remaining != nil:
		used = limit - float64(*detail.Remaining)
	default:
		return Limit{}, errors.New("remaining or used is required")
	}
	if used < 0 || used > limit*100 {
		return Limit{}, errors.New("usage is out of range")
	}
	reading := Limit{Window: window, Used: used / limit * 100}
	if detail.ResetTime != nil && *detail.ResetTime != "" {
		resetsAt, err := time.Parse(time.RFC3339Nano, *detail.ResetTime)
		if err != nil {
			return Limit{}, err
		}
		reading.ResetsAt = resetsAt.UTC()
	}
	return reading, nil
}
