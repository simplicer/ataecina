package domain_test

import (
	"errors"
	"math"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func TestWindowsAreRecognisedByDurationNeverByPosition(t *testing.T) {
	cases := []struct {
		minutes int
		kind    domain.WindowKind
	}{
		{300, domain.WindowFiveHours},
		{1440, domain.WindowDaily},
		{10080, domain.WindowWeekly},
		{15, domain.WindowOther},
		{43200, domain.WindowOther},
	}
	for _, c := range cases {
		window, err := domain.WindowOfMinutes(c.minutes)
		if err != nil {
			t.Fatalf("WindowOfMinutes(%d) = %v", c.minutes, err)
		}
		if window.Kind() != c.kind || window.Minutes() != c.minutes {
			t.Fatalf("WindowOfMinutes(%d) = %s/%d", c.minutes, window.Kind(), window.Minutes())
		}
	}
	if _, err := domain.WindowOfMinutes(0); !errors.Is(err, domain.ErrInvalidWindow) {
		t.Fatalf("WindowOfMinutes(0) = %v", err)
	}
}

func TestCalendarWindowsHaveNoFixedLength(t *testing.T) {
	for _, w := range []domain.Window{domain.MonthlyWindow(), domain.BillingCycleWindow()} {
		if w.Minutes() != 0 {
			t.Fatalf("%s has %d minutes", w.Kind(), w.Minutes())
		}
	}
	if domain.FiveHourWindow().Minutes() != 300 || domain.DailyWindow().Minutes() != 1440 || domain.WeeklyWindow().Minutes() != 10080 {
		t.Fatal("fixed windows have the wrong length")
	}
	if domain.MonthlyWindow().Kind() != domain.WindowMonthly || domain.BillingCycleWindow().Kind() != domain.WindowBillingCycle {
		t.Fatal("calendar windows have the wrong kind")
	}
}

func TestParseWindowRestoresEveryKind(t *testing.T) {
	cases := []struct {
		kind    string
		minutes int
		want    domain.Window
	}{
		{"FIVE_HOURS", 300, domain.FiveHourWindow()},
		{"DAILY", 1440, domain.DailyWindow()},
		{"WEEKLY", 10080, domain.WeeklyWindow()},
		{"MONTHLY", 0, domain.MonthlyWindow()},
		{"BILLING_CYCLE", 0, domain.BillingCycleWindow()},
	}
	for _, c := range cases {
		got, err := domain.ParseWindow(c.kind, c.minutes)
		if err != nil || got != c.want {
			t.Fatalf("ParseWindow(%s) = %v, %v", c.kind, got, err)
		}
	}
	other, err := domain.ParseWindow("OTHER", 90)
	if err != nil || other.Kind() != domain.WindowOther || other.Minutes() != 90 {
		t.Fatalf("ParseWindow(OTHER) = %v, %v", other, err)
	}
	for _, bad := range []struct {
		kind    string
		minutes int
	}{{"OTHER", 0}, {"HOURLY", 60}, {"FIVE_HOURS", 10}} {
		if _, err := domain.ParseWindow(bad.kind, bad.minutes); !errors.Is(err, domain.ErrInvalidWindow) {
			t.Fatalf("ParseWindow(%s, %d) = %v", bad.kind, bad.minutes, err)
		}
	}
}

func TestOtherWindowNeedsAPositiveLength(t *testing.T) {
	if _, err := domain.OtherWindow(-5); !errors.Is(err, domain.ErrInvalidWindow) {
		t.Fatalf("OtherWindow(-5) = %v", err)
	}
	w, err := domain.OtherWindow(60)
	if err != nil || w.Kind() != domain.WindowOther {
		t.Fatalf("OtherWindow(60) = %v, %v", w, err)
	}
}

func TestUsedPercentMayExceedAHundred(t *testing.T) {
	p, err := domain.NewUsedPercent(130.5)
	if err != nil || p.Float64() != 130.5 {
		t.Fatalf("NewUsedPercent(130.5) = %v, %v", p, err)
	}
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1), 10001} {
		if _, err := domain.NewUsedPercent(bad); !errors.Is(err, domain.ErrInvalidFigure) {
			t.Fatalf("NewUsedPercent(%v) = %v", bad, err)
		}
	}
	fraction, err := domain.UsedPercentFromFraction(0.235)
	if err != nil || math.Abs(fraction.Float64()-23.5) > 1e-9 {
		t.Fatalf("UsedPercentFromFraction = %v, %v", fraction, err)
	}
}
