package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

var noon = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func claudeSubscription() domain.Subscription {
	return domain.NewSubscription("claude", "anthropic", "claude-statusline", domain.FidelityReported, domain.NewPlan("pro"))
}

func percent(t *testing.T, v float64) domain.UsedPercent {
	t.Helper()
	p, err := domain.NewUsedPercent(v)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func okReading(t *testing.T, meters ...domain.Meter) domain.Reading {
	t.Helper()
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: noon, Meters: meters,
	})
	if err != nil {
		t.Fatalf("NewReading = %v", err)
	}
	return reading
}

func TestMeterIsRebornAfterItsReset(t *testing.T) {
	meter := domain.NewMeter("", domain.FiveHourWindow(), percent(t, 87), noon.Add(time.Hour))

	before := meter.At(noon.Add(59 * time.Minute))
	if before.Used() != 87 || before.Estimated() {
		t.Fatalf("before reset: %v", before)
	}
	after := meter.At(noon.Add(time.Hour))
	if after.Used() != 0 || !after.Estimated() || after.Window() != domain.FiveHourWindow() {
		t.Fatalf("after reset: %+v", after)
	}
	if _, known := after.ResetsAt(); known {
		t.Fatal("the next reset of a reborn window is unknown")
	}
	unknown := domain.NewMeter("", domain.WeeklyWindow(), percent(t, 10), time.Time{})
	if unknown.At(noon.Add(1000 * time.Hour)).Estimated() {
		t.Fatal("a meter without reset is never reborn")
	}
}

func TestReadingNeedsIdentityAndValidStates(t *testing.T) {
	base := domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeUnsupported,
	}
	if _, err := domain.NewReading(base); err != nil {
		t.Fatalf("valid reading: %v", err)
	}
	broken := []func(f *domain.ReadingFacts){
		func(f *domain.ReadingFacts) { f.Subscription = "" },
		func(f *domain.ReadingFacts) { f.Fidelity = "GUESSED" },
		func(f *domain.ReadingFacts) { f.Outcome = "FINE" },
		func(f *domain.ReadingFacts) { f.ErrorClass = "OOPS" },
		func(f *domain.ReadingFacts) { f.Outcome = domain.OutcomeOK },
		func(f *domain.ReadingFacts) {
			f.Meters = []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), 1, time.Time{})}
		},
		func(f *domain.ReadingFacts) {
			f.Outcome, f.ObservedAt = domain.OutcomeOK, noon
			m := domain.NewMeter("", domain.FiveHourWindow(), 1, time.Time{})
			f.Meters = []domain.Meter{m, m}
		},
	}
	for i, breakIt := range broken {
		facts := base
		breakIt(&facts)
		if _, err := domain.NewReading(facts); !errors.Is(err, domain.ErrInvalidReading) {
			t.Fatalf("case %d: NewReading = %v, want ErrInvalidReading", i, err)
		}
	}
}

func TestReadingCarriesEveryKindOfFigure(t *testing.T) {
	usd, _ := domain.NewCurrency("USD")
	spent := domain.NewMoney(mustAmount(t, "4"), usd)
	balance, _ := domain.NewBalance(nil, &spent)
	credits := domain.NewCredits(mustAmount(t, "3"), false)
	banked, _ := domain.NewBankedResets(1)
	five := domain.NewMeter("codex", domain.FiveHourWindow(), percent(t, 4), noon.Add(time.Hour))
	week := domain.NewMeter("codex", domain.WeeklyWindow(), percent(t, 41), noon.Add(48*time.Hour))
	premium := domain.NewMeter("premium", domain.WeeklyWindow(), percent(t, 120), time.Time{})
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "codex", Provider: "openai", Source: "codex-logs", Plan: domain.NewPlan("plus"),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomePartial, ObservedAt: noon,
		Meters: []domain.Meter{five, week, premium}, Balance: &balance, Credits: &credits, BankedResets: &banked,
		Hint: "  only\nsome  ",
	})
	if err != nil {
		t.Fatalf("NewReading = %v", err)
	}
	if got, _ := reading.Balance(); got != balance {
		t.Fatalf("Balance() = %v", got)
	}
	if got, _ := reading.Credits(); got != credits {
		t.Fatalf("Credits() = %v", got)
	}
	if got, _ := reading.BankedResets(); got.Available() != 1 {
		t.Fatalf("BankedResets() = %v", got)
	}
	if reading.Hint() != "only some" {
		t.Fatalf("Hint() = %q", reading.Hint())
	}
	if worst, _ := reading.WorstMeter(); worst.Bucket() != "premium" || worst.Used() != 120 {
		t.Fatalf("WorstMeter() = %+v", worst)
	}
	if len(reading.Meters()) != 3 || reading.Plan() != "PLUS" || reading.Provider() != "openai" || reading.Source() != "codex-logs" {
		t.Fatal("getters are wrong")
	}
	if observed, ok := reading.ObservedAt(); !ok || !observed.Equal(noon) {
		t.Fatalf("ObservedAt() = %v", observed)
	}
	reborn := reading.At(noon.Add(2 * time.Hour))
	if reborn.Meters()[0].Used() != 0 || !reborn.Meters()[0].Estimated() || reborn.Meters()[1].Used() != 41 {
		t.Fatalf("At() = %+v", reborn.Meters())
	}
}

func TestReadingWithoutFiguresHasNone(t *testing.T) {
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "zai", Provider: "zai", Source: "zai-api",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeUnsupported, Hint: strings.Repeat("x", 300),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reading.Balance(); ok {
		t.Fatal("no balance expected")
	}
	if _, ok := reading.Credits(); ok {
		t.Fatal("no credits expected")
	}
	if _, ok := reading.BankedResets(); ok {
		t.Fatal("no banked resets expected")
	}
	if _, ok := reading.WorstMeter(); ok {
		t.Fatal("no meter expected")
	}
	if _, ok := reading.ObservedAt(); ok {
		t.Fatal("never observed")
	}
	if reading.Freshness(noon, 5*time.Minute) != domain.FreshnessUnknown {
		t.Fatal("never observed is UNKNOWN")
	}
	if len([]rune(reading.Hint())) != 200 {
		t.Fatalf("hint not capped: %d", len([]rune(reading.Hint())))
	}
}

func TestSettleKeepsAFreshAnswerAndFillsTheDeclaredPlan(t *testing.T) {
	fresh := okReading(t, domain.NewMeter("", domain.FiveHourWindow(), percent(t, 23), noon.Add(time.Hour)))
	settled := domain.Settle(claudeSubscription(), nil, fresh, nil)
	if settled.Outcome() != domain.OutcomeOK || settled.Plan() != "PRO" {
		t.Fatalf("Settle = %s plan %q", settled.Outcome(), settled.Plan())
	}
	if _, waits := settled.RetryNotBefore(); waits {
		t.Fatal("a fresh answer clears the retry time")
	}
}

func TestSettleRejectsAnAnswerForAnotherSubscription(t *testing.T) {
	other := domain.NewSubscription("claude-work", "anthropic", "claude-statusline", domain.FidelityReported, "")
	fresh := okReading(t, domain.NewMeter("", domain.FiveHourWindow(), percent(t, 23), time.Time{}))
	settled := domain.Settle(other, nil, fresh, nil)
	if settled.Outcome() != domain.OutcomeBroken || settled.ErrorClass() != domain.ErrorContractChanged {
		t.Fatalf("Settle = %s/%s", settled.Outcome(), settled.ErrorClass())
	}
}

func TestSettleKeepsTheLastGoodReadingOnTransientFailures(t *testing.T) {
	previous := okReading(t, domain.NewMeter("", domain.FiveHourWindow(), percent(t, 50), noon.Add(time.Hour)))
	retry := noon.Add(10 * time.Minute)
	settled := domain.Settle(claudeSubscription(), &previous, domain.Reading{}, domain.RateLimited(retry, "slow down"))
	if settled.Outcome() != domain.OutcomeOK || settled.ErrorClass() != domain.ErrorRateLimited || settled.Hint() != "slow down" {
		t.Fatalf("Settle = %s/%s %q", settled.Outcome(), settled.ErrorClass(), settled.Hint())
	}
	if got, ok := settled.RetryNotBefore(); !ok || !got.Equal(retry) {
		t.Fatalf("RetryNotBefore() = %v", got)
	}
	if !settled.WaitsAt(noon.Add(5*time.Minute)) || settled.WaitsAt(retry) {
		t.Fatal("WaitsAt is wrong")
	}
	if settled.Meters()[0].Used() != 50 {
		t.Fatal("figures must be kept")
	}
}

func TestSettleWithoutAGoodReadingShowsTheFailure(t *testing.T) {
	cases := []struct {
		failure error
		outcome domain.Outcome
		class   domain.ErrorClass
	}{
		{domain.Failure(domain.ErrorUnavailable, "timeout", errors.New("dial")), domain.OutcomeUnavailable, domain.ErrorUnavailable},
		{domain.Failure(domain.ErrorUnreadable, "no logs", nil), domain.OutcomeUnavailable, domain.ErrorUnreadable},
		{domain.Failure(domain.ErrorKeyRejected, "key rejected", nil), domain.OutcomeKeyMissing, domain.ErrorKeyRejected},
		{domain.Failure(domain.ErrorContractChanged, "new shape", nil), domain.OutcomeBroken, domain.ErrorContractChanged},
		{domain.Unsupported("planned"), domain.OutcomeUnsupported, ""},
		{domain.KeyMissing("set a key"), domain.OutcomeKeyMissing, ""},
		{domain.KeysLocked("unlock"), domain.OutcomeLocked, ""},
		{errors.New("boom"), domain.OutcomeBroken, ""},
	}
	failedBefore, _ := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeBroken,
	})
	for _, c := range cases {
		settled := domain.Settle(claudeSubscription(), &failedBefore, domain.Reading{}, c.failure)
		if settled.Outcome() != c.outcome || settled.ErrorClass() != c.class {
			t.Fatalf("%v: Settle = %s/%s", c.failure, settled.Outcome(), settled.ErrorClass())
		}
		if settled.Plan() != "PRO" || settled.Fidelity() != domain.FidelityReported || len(settled.Meters()) != 0 {
			t.Fatalf("%v: failed reading lost its subscription facts", c.failure)
		}
	}
}

func TestSourceFailureDescribesItself(t *testing.T) {
	cause := errors.New("connection refused")
	failure := domain.Failure(domain.ErrorUnavailable, "provider down", cause)
	if failure.Error() != "UNAVAILABLE: provider down" || !errors.Is(failure, cause) {
		t.Fatalf("Error() = %q", failure.Error())
	}
	if failure.Class() != domain.ErrorUnavailable || failure.Outcome() != domain.OutcomeUnavailable || failure.Hint() != "provider down" {
		t.Fatal("getters are wrong")
	}
	if _, ok := failure.RetryAt(); ok {
		t.Fatal("no retry time expected")
	}
	if domain.Unsupported("").Error() != "UNSUPPORTED" {
		t.Fatalf("Error() = %q", domain.Unsupported("").Error())
	}
}

func TestSnapshotHoldsOneReadingPerSubscription(t *testing.T) {
	reading := okReading(t, domain.NewMeter("", domain.FiveHourWindow(), percent(t, 95), noon.Add(time.Hour)))
	if _, err := domain.NewSnapshot(noon, 5*time.Minute, []domain.Reading{reading, reading}); !errors.Is(err, domain.ErrInvalidSnapshot) {
		t.Fatalf("duplicate subscription = %v", err)
	}
	if _, err := domain.NewSnapshot(time.Time{}, 5*time.Minute, nil); !errors.Is(err, domain.ErrInvalidSnapshot) {
		t.Fatalf("no time = %v", err)
	}
	if _, err := domain.NewSnapshot(noon, 0, nil); !errors.Is(err, domain.ErrInvalidSnapshot) {
		t.Fatalf("no interval = %v", err)
	}
	snapshot, err := domain.NewSnapshot(noon, 5*time.Minute, []domain.Reading{reading})
	if err != nil {
		t.Fatal(err)
	}
	if taken, ok := snapshot.TakenAt(); !ok || !taken.Equal(noon) || snapshot.Interval() != 5*time.Minute {
		t.Fatal("snapshot getters are wrong")
	}
	if got, ok := snapshot.ReadingOf("claude"); !ok || got.Subscription() != "claude" {
		t.Fatal("ReadingOf(claude) not found")
	}
	if _, ok := snapshot.ReadingOf("codex"); ok {
		t.Fatal("ReadingOf(codex) must be absent")
	}
	if len(snapshot.Readings()) != 1 {
		t.Fatal("Readings() is wrong")
	}
}

func TestStatusReportAppliesFreshnessAndRebirth(t *testing.T) {
	reading := okReading(t,
		domain.NewMeter("", domain.FiveHourWindow(), percent(t, 95), noon.Add(time.Hour)),
		domain.NewMeter("", domain.WeeklyWindow(), percent(t, 41), noon.Add(72*time.Hour)),
	)
	snapshot, _ := domain.NewSnapshot(noon, 5*time.Minute, []domain.Reading{reading})

	early := snapshot.StatusAt(noon.Add(time.Minute))
	if worst, ok := early.WorstUsed(); !ok || worst != 95 {
		t.Fatalf("WorstUsed() = %v", worst)
	}
	if early.Statuses()[0].Freshness() != domain.FreshnessFresh {
		t.Fatal("one minute old must be FRESH")
	}
	late := snapshot.StatusAt(noon.Add(2 * time.Hour))
	status := late.Statuses()[0]
	if status.Freshness() != domain.FreshnessStale {
		t.Fatal("two hours old must be STALE")
	}
	if worst, _ := late.WorstUsed(); worst != 41 {
		t.Fatalf("after the five-hour rebirth the worst is the week: %v", worst)
	}
	if !status.Reading().Meters()[0].Estimated() {
		t.Fatal("the reborn window must be estimated")
	}
	if taken, ok := late.TakenAt(); !ok || !taken.Equal(noon) {
		t.Fatal("TakenAt() is wrong")
	}
	empty := domain.Snapshot{}.StatusAt(noon)
	if _, ok := empty.WorstUsed(); ok || len(empty.Statuses()) != 0 {
		t.Fatal("an empty snapshot shows nothing")
	}
	if _, ok := empty.TakenAt(); ok {
		t.Fatal("an empty snapshot was never taken")
	}
}
