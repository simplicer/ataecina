package catalog_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
)

func TestPeakHoursComeWithThePlansThatPublishThem(t *testing.T) {
	sources := catalog.Standard()
	// Friday 2026-11-06 09:00 UTC is 17:00 UTC+8 and 01:00 Pacific.
	friday := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	cases := map[domain.SourceID]struct {
		current domain.RateKind
		benefit domain.OffPeakBenefit
	}{
		"zai-api":     {domain.RatePeak, domain.OffPeakHalfCredits},
		"alibaba-cli": {domain.RatePeak, domain.OffPeakDiscountedCredits},
	}
	for source, want := range cases {
		entry, ok := sources.Lookup(source)
		if !ok || entry.PeakHours == nil {
			t.Fatalf("%s has no peak hours", source)
		}
		if period := entry.PeakHours.At(friday); period.Current != want.current || period.Benefit != want.benefit {
			t.Fatalf("%s = %+v", source, period)
		}
	}
	if claude, _ := sources.Lookup("claude-statusline"); claude.PeakHours != nil {
		if period := claude.PeakHours.At(friday); period.Current != domain.RateOffPeak || period.Benefit != domain.OffPeakLongerSessions {
			t.Fatalf("claude = %+v", period)
		}
	}
	for _, source := range []domain.SourceID{"minimax-api", "alibaba-coding-cli", "groq-api"} {
		if entry, _ := sources.Lookup(source); entry.PeakHours != nil {
			t.Fatalf("%s has peak hours nobody published", source)
		}
	}
}
