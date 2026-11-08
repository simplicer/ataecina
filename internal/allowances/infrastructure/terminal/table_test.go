package terminal_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
)

var taken = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func mustReading(t *testing.T, facts domain.ReadingFacts) domain.Reading {
	t.Helper()
	reading, err := domain.NewReading(facts)
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

func render(t *testing.T, now time.Time, colourful bool, readings ...domain.Reading) string {
	t.Helper()
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, readings)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := terminal.Render(&out, snapshot.StatusAt(now), now, nil, colourful); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func sample(t *testing.T) []domain.Reading {
	usd, _ := domain.NewCurrency("USD")
	eur, _ := domain.NewCurrency("EUR")
	cny, _ := domain.NewCurrency("CNY")
	amount := func(raw string) domain.Amount { a, _ := domain.ParseAmount(raw); return a }
	derivedLeft, derivedSpent := domain.NewMoney(amount("3.1"), usd), domain.NewMoney(amount("16.9"), usd)
	derived, _ := domain.NewBalance(&derivedLeft, &derivedSpent)
	euros := domain.NewMoney(amount("5"), eur)
	eurBalance, _ := domain.NewBalance(&euros, nil)
	yuan := domain.NewMoney(amount("7"), cny)
	cnyBalance, _ := domain.NewBalance(nil, &yuan)
	credits := domain.NewCredits(amount("0"), false)
	unlimited := domain.NewCredits(amount("0"), true)
	banked, _ := domain.NewBankedResets(2)
	ninety, _ := domain.OtherWindow(90)
	twoDays, _ := domain.OtherWindow(2880)
	sixHours, _ := domain.OtherWindow(360)
	return []domain.Reading{
		mustReading(t, domain.ReadingFacts{
			Subscription: "claude", Provider: "anthropic", Source: "claude-statusline", Plan: "PRO",
			Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken.Add(-2 * time.Minute),
			Meters: []domain.Meter{
				domain.NewMeter("", domain.FiveHourWindow(), 23, taken.Add(130*time.Minute)),
				domain.NewMeter("", domain.WeeklyWindow(), 41.25, taken.Add(108*time.Hour)),
			},
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "codex", Provider: "openai", Source: "codex-logs", Plan: "PLUS",
			Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken.Add(-3 * time.Hour),
			Meters: []domain.Meter{
				domain.NewMeter("codex", domain.FiveHourWindow(), 96, taken.Add(-time.Minute)),
				domain.NewMeter("premium", domain.WeeklyWindow(), 91, taken.Add(30*time.Second)),
				domain.NewMeter("premium", domain.DailyWindow(), 1, time.Time{}),
				domain.NewMeter("premium", domain.MonthlyWindow(), 1, time.Time{}),
				domain.NewMeter("premium", domain.BillingCycleWindow(), 1, time.Time{}),
				domain.NewMeter("premium", ninety, 1, time.Time{}),
				domain.NewMeter("premium", twoDays, 1, time.Time{}),
				domain.NewMeter("premium", sixHours, 1, time.Time{}),
			},
			Credits: &credits, BankedResets: &banked, ErrorClass: domain.ErrorUnavailable,
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "zen", Provider: "opencode", Source: "opencode-zen",
			Fidelity: domain.FidelityDerived, Outcome: domain.OutcomeOK, ObservedAt: taken.Add(-72 * time.Hour), Balance: &derived,
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "eu", Provider: "vercel", Source: "vercel-api",
			Fidelity: domain.FidelityReported, Outcome: domain.OutcomePartial, ObservedAt: taken, Balance: &eurBalance, Credits: &unlimited,
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "cn", Provider: "alibaba", Source: "alibaba-cli",
			Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken.Add(-30 * time.Second), Balance: &cnyBalance,
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "minimax", Provider: "minimax", Source: "minimax-api",
			Fidelity: domain.FidelityReported, Outcome: domain.OutcomeBroken, ErrorClass: domain.ErrorContractChanged, Hint: "unexpected shape",
		}),
		mustReading(t, domain.ReadingFacts{
			Subscription: "groq", Provider: "groq", Source: "heimdall",
			Fidelity: domain.FidelityObserved, Outcome: domain.OutcomeUnsupported,
		}),
	}
}

func TestTheTableShowsEverySubscription(t *testing.T) {
	out := render(t, taken, false, sample(t)...)
	for _, want := range []string{
		"Ataecina · updated just now",
		"SUBSCRIPTION",
		"5h 23% ↻ 2h10m · week 41.3% ↻ 4d12h",
		"5h 0% (reborn)",
		"premium week 91% ↻ <1m",
		"premium day 1%", "premium month 1%", "premium cycle 1%", "premium 90m 1%", "premium 2d 1%", "premium 6h 1%",
		"credits 0 · 2 banked resets",
		"stale 3h ago · last try unavailable",
		"≈ $3.10 left · $16.90 spent",
		"derived",
		"3d ago",
		"€5.00 left · credits ∞",
		"partial",
		"7.00 CNY spent",
		"broken: unexpected shape",
		"unsupported · observed",
		"fresh 2m ago",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("no colours without a terminal")
	}
}

func TestColoursMarkSeverityOnlyOnATerminal(t *testing.T) {
	out := render(t, taken.Add(-2*time.Minute), true, sample(t)...)
	for _, sgr := range []string{"\x1b[31m", "\x1b[2m"} {
		if !strings.Contains(out, sgr) {
			t.Fatalf("missing %q", sgr)
		}
	}
	yellow := render(t, taken, true, mustReading(t, domain.ReadingFacts{
		Subscription: "zai", Provider: "zai", Source: "zai-api", Fidelity: domain.FidelityReported,
		Outcome: domain.OutcomeOK, ObservedAt: taken, Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), 92, time.Time{})},
	}))
	if !strings.Contains(yellow, "\x1b[33m92%") {
		t.Fatalf("a meter over the first threshold is yellow:\n%q", yellow)
	}
}

func TestBeforeTheFirstCollection(t *testing.T) {
	var out bytes.Buffer
	if err := terminal.Render(&out, domain.Snapshot{}.StatusAt(taken), taken, nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No snapshot yet") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestAgeOfAnOldSnapshot(t *testing.T) {
	out := render(t, taken.Add(26*time.Minute), false)
	if !strings.HasPrefix(out, "Ataecina · updated 26m ago") {
		t.Fatalf("out = %q", out)
	}
}

func TestTheTableSaysWhenOffPeakEnds(t *testing.T) {
	local := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = local })
	readings := sample(t)
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, readings)
	if err != nil {
		t.Fatal(err)
	}
	first := readings[0].Subscription().String()
	end := taken.Add(2 * time.Hour)
	periods := map[string]domain.RatePeriod{
		first: {Current: domain.RateOffPeak, EndsAt: taken.Add(time.Hour), PeakStartsAt: taken.Add(time.Hour), PeakEndsAt: end, Benefit: domain.OffPeakHalfCredits},
	}
	var out bytes.Buffer
	if err := terminal.Render(&out, snapshot.StatusAt(taken), taken, periods, false); err != nil {
		t.Fatal(err)
	}
	if want := "off-peak until " + taken.Add(time.Hour).Format("15:04"); !strings.Contains(out.String(), want) {
		t.Fatalf("missing %q:\n%s", want, out.String())
	}
	periods[first] = domain.RatePeriod{Current: domain.RatePeak, EndsAt: taken.Add(72 * time.Hour), Benefit: domain.OffPeakHalfCredits}
	out.Reset()
	if err := terminal.Render(&out, snapshot.StatusAt(taken), taken, periods, false); err != nil {
		t.Fatal(err)
	}
	if want := "peak until " + taken.Add(72*time.Hour).Format("Mon 15:04"); !strings.Contains(out.String(), want) {
		t.Fatalf("missing %q:\n%s", want, out.String())
	}
}
