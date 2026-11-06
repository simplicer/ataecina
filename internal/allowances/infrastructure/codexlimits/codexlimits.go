// Package codexlimits is what both Codex sources read — the session logs
// and the app server — and how it maps to an allowance reading: windows
// by duration (never by position), one bucket per limit id, credits, plan
// and banked resets (SPECS §3.2).
package codexlimits

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// MainLimit is the bucket of Codex's own plan windows.
const MainLimit = "codex"

// ErrNoFigures reports limits without any window, credits or banked resets.
var ErrNoFigures = errors.New("Codex has not reported any window or credits yet")

// Limits is one rate-limits snapshot for one limit id.
type Limits struct {
	ObservedAt time.Time `json:"observed_at"`
	LimitID    string    `json:"limit_id"`
	Plan       string    `json:"plan,omitempty"`
	Windows    []Window  `json:"windows"`
	Credits    *Credits  `json:"credits,omitempty"`
}

// Window is one Codex window: its length, the percentage used and the
// reset instant (zero when unknown).
type Window struct {
	Minutes     int       `json:"minutes"`
	UsedPercent float64   `json:"used_percent"`
	ResetsAt    time.Time `json:"resets_at"`
}

// errInvalidWindow reports a window that cannot exist.
var errInvalidWindow = errors.New("invalid Codex window")

// NewWindow validates a window as Codex reported it: a positive length and
// a finite, non-negative percentage. A zero resetsAt means unknown.
func NewWindow(minutes int, usedPercent float64, resetsAt time.Time) (Window, error) {
	if minutes <= 0 || math.IsNaN(usedPercent) || math.IsInf(usedPercent, 0) || usedPercent < 0 {
		return Window{}, errInvalidWindow
	}
	return Window{Minutes: minutes, UsedPercent: usedPercent, ResetsAt: resetsAt.UTC()}, nil
}

// Credits are Codex credits; Balance is a decimal string.
type Credits struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance,omitempty"`
}

// Reading builds the subscription's reading from the latest limits of
// each limit id. The main bucket comes first; other buckets are shown only
// when they carry windows. banked is the number of banked resets, or nil
// when the source does not know it.
func Reading(subscription domain.Subscription, source domain.SourceID, latest map[string]Limits, banked *int) (domain.Reading, error) {
	main, hasMain := latest[MainLimit]
	ids := make([]string, 0, len(latest))
	for id, limits := range latest {
		if id != MainLimit && len(limits.Windows) > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if hasMain {
		ids = append([]string{MainLimit}, ids...)
	}
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: source,
		Plan: domain.NewPlan(main.Plan), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK,
	}
	for _, id := range ids {
		limits := latest[id]
		if limits.ObservedAt.After(facts.ObservedAt) {
			facts.ObservedAt = limits.ObservedAt
		}
		meters, err := meters(limits)
		if err != nil {
			return domain.Reading{}, err
		}
		facts.Meters = append(facts.Meters, meters...)
	}
	if credits := main.Credits; hasMain && credits != nil && (credits.HasCredits || credits.Unlimited) {
		available := "0"
		if credits.Balance != "" {
			available = credits.Balance
		}
		amount, err := domain.ParseAmount(available)
		if err != nil {
			return domain.Reading{}, fmt.Errorf("credits balance: %w", err)
		}
		value := domain.NewCredits(amount, credits.Unlimited)
		facts.Credits = &value
	}
	if banked != nil {
		resets, err := domain.NewBankedResets(*banked)
		if err != nil {
			return domain.Reading{}, err
		}
		facts.BankedResets = &resets
	}
	if len(facts.Meters) == 0 && facts.Credits == nil && facts.BankedResets == nil {
		return domain.Reading{}, ErrNoFigures
	}
	return domain.NewReading(facts)
}

func meters(limits Limits) ([]domain.Meter, error) {
	bucket := domain.NewBucketID(limits.LimitID)
	seen := make(map[int]bool, len(limits.Windows))
	meters := make([]domain.Meter, 0, len(limits.Windows))
	for _, window := range limits.Windows {
		if seen[window.Minutes] {
			continue
		}
		seen[window.Minutes] = true
		span, err := domain.WindowOfMinutes(window.Minutes)
		if err != nil {
			return nil, err
		}
		used, err := domain.NewUsedPercent(window.UsedPercent)
		if err != nil {
			return nil, err
		}
		meters = append(meters, domain.NewMeter(bucket, span, used, window.ResetsAt))
	}
	return meters, nil
}
