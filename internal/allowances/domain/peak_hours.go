package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidPeakHours reports peak hours that cannot exist.
var ErrInvalidPeakHours = errors.New("invalid peak hours")

// OffPeakBenefit is what the same use saves outside the peak hours.
type OffPeakBenefit string

// Off-peak benefits (SPECS §13).
const (
	// OffPeakHalfCredits: off-peak use draws half the credits (Z.ai).
	OffPeakHalfCredits OffPeakBenefit = "HALF_CREDITS"
	// OffPeakDiscountedCredits: off-peak use draws fewer credits, by a
	// discount that depends on the model (Alibaba Token Plan).
	OffPeakDiscountedCredits OffPeakBenefit = "DISCOUNTED_CREDITS"
	// OffPeakLongerSessions: the session window drains slower off-peak;
	// the weekly window does not change (Claude).
	OffPeakLongerSessions OffPeakBenefit = "LONGER_SESSIONS"
)

// RateKind is whether an instant falls in the peak hours or off-peak.
type RateKind string

// Rate kinds.
const (
	RatePeak    RateKind = "PEAK"
	RateOffPeak RateKind = "OFF_PEAK"
)

// PeakHours are the hours a provider charges its allowance at the full
// rate, the same span on every day it names, in the provider's own time
// zone; the rest of the week is off-peak, when the same use costs less.
type PeakHours struct {
	zone    *time.Location
	days    [7]bool
	from    int
	to      int
	benefit OffPeakBenefit
}

// RatePeriod is where an instant falls: the period, when it ends, the peak
// that holds it or comes next, and what off-peak saves.
type RatePeriod struct {
	Current      RateKind
	EndsAt       time.Time
	PeakStartsAt time.Time
	PeakEndsAt   time.Time
	Benefit      OffPeakBenefit
}

// NewPeakHours makes peak hours from minute from to minute to after
// midnight (0 ≤ from < to ≤ 1440) on each of days, in zone.
func NewPeakHours(zone *time.Location, days []time.Weekday, from, to int, benefit OffPeakBenefit) (PeakHours, error) {
	switch {
	case zone == nil:
		return PeakHours{}, fmt.Errorf("%w: no time zone", ErrInvalidPeakHours)
	case len(days) == 0:
		return PeakHours{}, fmt.Errorf("%w: no day", ErrInvalidPeakHours)
	case from < 0 || to > dayMinutes || from >= to:
		return PeakHours{}, fmt.Errorf("%w: from %d to %d minutes", ErrInvalidPeakHours, from, to)
	}
	switch benefit {
	case OffPeakHalfCredits, OffPeakDiscountedCredits, OffPeakLongerSessions:
	default:
		return PeakHours{}, fmt.Errorf("%w: benefit %q", ErrInvalidPeakHours, benefit)
	}
	hours := PeakHours{zone: zone, from: from, to: to, benefit: benefit}
	for _, day := range days {
		if day < time.Sunday || day > time.Saturday {
			return PeakHours{}, fmt.Errorf("%w: day %d", ErrInvalidPeakHours, day)
		}
		hours.days[day] = true
	}
	return hours, nil
}

// At tells where now falls. A peak never spans midnight, so the one that
// holds now started today; the next one comes within a week.
func (p PeakHours) At(now time.Time) RatePeriod {
	local := now.In(p.zone)
	for offset := 0; offset <= 7; offset++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 0, 0, 0, 0, p.zone)
		if !p.days[day.Weekday()] {
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), p.from/60, p.from%60, 0, 0, p.zone)
		end := time.Date(day.Year(), day.Month(), day.Day(), p.to/60, p.to%60, 0, 0, p.zone)
		if !now.Before(end) {
			continue
		}
		period := RatePeriod{Current: RateOffPeak, EndsAt: start, PeakStartsAt: start, PeakEndsAt: end, Benefit: p.benefit}
		if !now.Before(start) {
			period.Current, period.EndsAt = RatePeak, end
		}
		return period
	}
	return RatePeriod{}
}
