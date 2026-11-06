package domain

import (
	"errors"
	"fmt"
	"math"
)

// ErrInvalidWindow reports a window that cannot exist.
var ErrInvalidWindow = errors.New("invalid window")

// ErrInvalidFigure reports a percentage or amount that cannot be shown.
var ErrInvalidFigure = errors.New("invalid figure")

// WindowKind is the span a meter covers.
type WindowKind string

// Window kinds (SPECS §1).
const (
	WindowFiveHours    WindowKind = "FIVE_HOURS"
	WindowDaily        WindowKind = "DAILY"
	WindowWeekly       WindowKind = "WEEKLY"
	WindowMonthly      WindowKind = "MONTHLY"
	WindowBillingCycle WindowKind = "BILLING_CYCLE"
	WindowOther        WindowKind = "OTHER"
)

const (
	fiveHourMinutes = 300
	dayMinutes      = 1440
	weekMinutes     = 10080
)

// Window is the span of one meter. Calendar windows (monthly, billing
// cycle) have no fixed length and report zero minutes.
type Window struct {
	kind    WindowKind
	minutes int
}

// FiveHourWindow is the rolling five-hour window.
func FiveHourWindow() Window { return Window{WindowFiveHours, fiveHourMinutes} }

// DailyWindow is a one-day window.
func DailyWindow() Window { return Window{WindowDaily, dayMinutes} }

// WeeklyWindow is a seven-day window.
func WeeklyWindow() Window { return Window{WindowWeekly, weekMinutes} }

// MonthlyWindow is a calendar month.
func MonthlyWindow() Window { return Window{WindowMonthly, 0} }

// BillingCycleWindow is the provider's billing period.
func BillingCycleWindow() Window { return Window{WindowBillingCycle, 0} }

// OtherWindow is any other fixed length, in minutes.
func OtherWindow(minutes int) (Window, error) {
	if minutes <= 0 {
		return Window{}, fmt.Errorf("%w: %d minutes", ErrInvalidWindow, minutes)
	}
	return Window{WindowOther, minutes}, nil
}

// WindowOfMinutes recognises a window by its length: 300 is five hours,
// 1440 a day, 10080 a week, anything else OTHER. Providers that send
// windows in a list are mapped by duration, never by position.
func WindowOfMinutes(minutes int) (Window, error) {
	switch minutes {
	case fiveHourMinutes:
		return FiveHourWindow(), nil
	case dayMinutes:
		return DailyWindow(), nil
	case weekMinutes:
		return WeeklyWindow(), nil
	}
	return OtherWindow(minutes)
}

// ParseWindow restores a window from its kind and length.
func ParseWindow(kind string, minutes int) (Window, error) {
	var window Window
	switch WindowKind(kind) {
	case WindowFiveHours:
		window = FiveHourWindow()
	case WindowDaily:
		window = DailyWindow()
	case WindowWeekly:
		window = WeeklyWindow()
	case WindowMonthly:
		return MonthlyWindow(), nil
	case WindowBillingCycle:
		return BillingCycleWindow(), nil
	case WindowOther:
		return OtherWindow(minutes)
	default:
		return Window{}, fmt.Errorf("%w: kind %q", ErrInvalidWindow, kind)
	}
	if minutes != window.minutes {
		return Window{}, fmt.Errorf("%w: %s with %d minutes", ErrInvalidWindow, kind, minutes)
	}
	return window, nil
}

// Kind is the window kind.
func (w Window) Kind() WindowKind { return w.kind }

// Minutes is the fixed length, or 0 for calendar windows.
func (w Window) Minutes() int { return w.minutes }

// UsedPercent is the share of a window already spent. It may exceed 100
// when the provider allows overage.
type UsedPercent float64

const maxUsedPercent = 10000

// NewUsedPercent validates a percentage reported by a provider.
func NewUsedPercent(value float64) (UsedPercent, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxUsedPercent {
		return 0, fmt.Errorf("%w: used percentage %v", ErrInvalidFigure, value)
	}
	return UsedPercent(value), nil
}

// UsedPercentFromFraction converts a 0–1 fraction of the cap.
func UsedPercentFromFraction(fraction float64) (UsedPercent, error) {
	return NewUsedPercent(fraction * 100)
}

// Float64 is the percentage as a number.
func (p UsedPercent) Float64() float64 { return float64(p) }
