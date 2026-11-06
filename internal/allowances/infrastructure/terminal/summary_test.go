package terminal_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
)

func TestTheSummaryFitsAStatusLine(t *testing.T) {
	reading := mustReading(t, domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{
			domain.NewMeter("", domain.FiveHourWindow(), 23.4, taken.Add(130*time.Minute)),
			domain.NewMeter("", domain.WeeklyWindow(), 41.2, taken.Add(108*time.Hour)),
			domain.NewMeter("spend_limit", domain.BillingCycleWindow(), 12, time.Time{}),
		},
	})
	if got := terminal.Summary(reading, taken); got != "5h 23% · 7d 41% · spend_limit cycle 12% · ↻ 2h10m" {
		t.Fatalf("Summary = %q", got)
	}
	if got := terminal.Summary(reading, taken.Add(3*time.Hour)); got != "5h 0% · 7d 41% · spend_limit cycle 12% · ↻ 4d09h" {
		t.Fatalf("after the five-hour rebirth: %q", got)
	}
	if got := terminal.Summary(domain.Reading{}, taken); got != terminal.NothingYet {
		t.Fatalf("Summary(nothing) = %q", got)
	}
}
