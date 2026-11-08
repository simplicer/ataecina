package domain_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

var (
	utc8     = time.FixedZone("UTC+8", 8*3600)
	weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
)

func peakHours(t *testing.T, zone *time.Location, days []time.Weekday, from, to int) domain.PeakHours {
	t.Helper()
	hours, err := domain.NewPeakHours(zone, days, from, to, domain.OffPeakHalfCredits)
	if err != nil {
		t.Fatal(err)
	}
	return hours
}

func TestWeekdayPeakHours(t *testing.T) {
	// Monday to Friday, 14:00 to 18:00 UTC+8 (06:00 to 10:00 UTC).
	hours := peakHours(t, utc8, weekdays, 14*60, 18*60)
	at := func(text string) time.Time {
		instant, err := time.Parse(time.RFC3339, text)
		if err != nil {
			t.Fatal(err)
		}
		return instant
	}
	cases := []struct {
		now, ends, peakStarts string
		current               domain.RateKind
	}{
		{"2026-11-09T05:59:00Z", "2026-11-09T06:00:00Z", "2026-11-09T06:00:00Z", domain.RateOffPeak}, // Monday, before
		{"2026-11-09T06:00:00Z", "2026-11-09T10:00:00Z", "2026-11-09T06:00:00Z", domain.RatePeak},    // the first minute
		{"2026-11-09T09:59:59Z", "2026-11-09T10:00:00Z", "2026-11-09T06:00:00Z", domain.RatePeak},
		{"2026-11-09T10:00:00Z", "2026-11-10T06:00:00Z", "2026-11-10T06:00:00Z", domain.RateOffPeak}, // the end is off-peak
		{"2026-11-13T12:00:00Z", "2026-11-16T06:00:00Z", "2026-11-16T06:00:00Z", domain.RateOffPeak}, // Friday evening: the weekend
		{"2026-11-08T07:00:00Z", "2026-11-09T06:00:00Z", "2026-11-09T06:00:00Z", domain.RateOffPeak}, // Sunday at peak time
	}
	for _, c := range cases {
		period := hours.At(at(c.now))
		if period.Current != c.current || !period.EndsAt.Equal(at(c.ends)) || !period.PeakStartsAt.Equal(at(c.peakStarts)) {
			t.Fatalf("%s: %+v", c.now, period)
		}
		if period.PeakEndsAt.Sub(period.PeakStartsAt) != 4*time.Hour || period.Benefit != domain.OffPeakHalfCredits {
			t.Fatalf("%s: peak %v to %v", c.now, period.PeakStartsAt, period.PeakEndsAt)
		}
	}
}

func TestPeakHoursFollowDaylightSaving(t *testing.T) {
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("no time zone database:", err)
	}
	hours := peakHours(t, pacific, weekdays, 5*60, 11*60)
	// 05:00 Pacific is 12:00 UTC in summer time and 13:00 UTC in winter.
	summer := hours.At(time.Date(2026, 10, 30, 11, 0, 0, 0, time.UTC))
	winter := hours.At(time.Date(2026, 11, 6, 11, 0, 0, 0, time.UTC))
	if summer.EndsAt.UTC().Hour() != 12 || winter.EndsAt.UTC().Hour() != 13 {
		t.Fatalf("summer %v, winter %v", summer.EndsAt.UTC(), winter.EndsAt.UTC())
	}
}

func TestEveryDayPeakHours(t *testing.T) {
	// Off-peak 22:00 to 08:00 UTC+8 is peak 08:00 to 22:00 every day.
	all := []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}
	hours := peakHours(t, utc8, all, 8*60, 22*60)
	night := hours.At(time.Date(2026, 11, 7, 23, 0, 0, 0, utc8))
	if night.Current != domain.RateOffPeak || !night.EndsAt.Equal(time.Date(2026, 11, 8, 8, 0, 0, 0, utc8)) {
		t.Fatalf("night = %+v", night)
	}
	whole := peakHours(t, utc8, all, 0, 24*60)
	if late := whole.At(time.Date(2026, 11, 7, 23, 30, 0, 0, utc8)); late.Current != domain.RatePeak || !late.EndsAt.Equal(time.Date(2026, 11, 8, 0, 0, 0, 0, utc8)) {
		t.Fatalf("a whole-day peak = %+v", late)
	}
}

func TestImpossiblePeakHoursAreRefused(t *testing.T) {
	cases := []struct {
		zone     *time.Location
		days     []time.Weekday
		from, to int
		benefit  domain.OffPeakBenefit
	}{
		{nil, weekdays, 60, 120, domain.OffPeakHalfCredits},
		{utc8, nil, 60, 120, domain.OffPeakHalfCredits},
		{utc8, weekdays, 120, 60, domain.OffPeakHalfCredits},
		{utc8, weekdays, -1, 60, domain.OffPeakHalfCredits},
		{utc8, weekdays, 60, 1441, domain.OffPeakHalfCredits},
		{utc8, []time.Weekday{7}, 60, 120, domain.OffPeakHalfCredits},
		{utc8, weekdays, 60, 120, "FREE_LUNCH"},
	}
	for i, c := range cases {
		if _, err := domain.NewPeakHours(c.zone, c.days, c.from, c.to, c.benefit); !errors.Is(err, domain.ErrInvalidPeakHours) {
			t.Fatalf("case %d accepted: %v", i, err)
		}
	}
}
