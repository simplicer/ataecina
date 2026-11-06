package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

var policy = application.CollectionPolicy{Interval: 5 * time.Minute, Deadline: 2 * time.Second}

func subscription(id string) domain.Subscription {
	return domain.NewSubscription(domain.SubscriptionID(id), "anthropic", "claude-statusline", domain.FidelityReported, domain.NewPlan("pro"))
}

func meterReading(t *testing.T, id string, used float64, observed time.Time) domain.Reading {
	t.Helper()
	p, err := domain.NewUsedPercent(used)
	if err != nil {
		t.Fatal(err)
	}
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: domain.SubscriptionID(id), Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: observed,
		Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), p, observed.Add(3*time.Hour))},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

func answering(reading domain.Reading) domain.Source {
	return sourceFunc(func(context.Context, domain.Subscription) (domain.Reading, error) { return reading, nil })
}

func failing(err error) domain.Source {
	return sourceFunc(func(context.Context, domain.Subscription) (domain.Reading, error) { return domain.Reading{}, err })
}

func collect(t *testing.T, subscriptions []domain.Subscription, sources catalog, shelf *snapshotShelf, policy application.CollectionPolicy) (domain.Snapshot, *journal, error) {
	t.Helper()
	j := &journal{}
	collector := application.NewCollectReadings(subscriptions, policy, application.CollectReadingsPorts{
		Sources: sources, Previous: shelf, Store: shelf, Journal: j, Clock: fixedClock{now},
	})
	snapshot, err := collector.Collect(context.Background())
	return snapshot, j, err
}

func TestCollectAsksEverySourceAndKeepsTheSnapshot(t *testing.T) {
	shelf := &snapshotShelf{readErr: domain.ErrNoSnapshot}
	sources := catalog{
		"claude": answering(meterReading(t, "claude", 23, now)),
		"zai":    failing(domain.Failure(domain.ErrorContractChanged, "unexpected shape", nil)),
	}
	snapshot, j, err := collect(t, []domain.Subscription{subscription("claude"), subscription("zai"), subscription("minimax")}, sources, shelf, policy)
	if err != nil {
		t.Fatalf("Collect = %v", err)
	}
	if shelf.kept == nil || shelf.keepCalled != 1 {
		t.Fatal("the snapshot was not kept")
	}
	readings := snapshot.Readings()
	if len(readings) != 3 {
		t.Fatalf("%d readings", len(readings))
	}
	want := []domain.Outcome{domain.OutcomeOK, domain.OutcomeBroken, domain.OutcomeUnsupported}
	for i, reading := range readings {
		if reading.Outcome() != want[i] {
			t.Fatalf("reading %d (%s) = %s, want %s", i, reading.Subscription(), reading.Outcome(), want[i])
		}
	}
	if readings[0].Plan() != "PRO" {
		t.Fatal("the declared plan fills an unknown reported plan")
	}
	if taken, _ := snapshot.TakenAt(); !taken.Equal(now) || snapshot.Interval() != policy.Interval {
		t.Fatal("snapshot time or interval is wrong")
	}
	if len(j.records) != 3 || !j.records[0].Asked || j.records[2].Asked {
		t.Fatalf("journal = %+v", j.records)
	}
	if j.records[0].Freshness != domain.FreshnessFresh || j.records[2].Freshness != domain.FreshnessUnknown {
		t.Fatal("journal freshness is wrong")
	}
}

func TestCollectKeepsTheLastGoodReadingWhenASourceIsDown(t *testing.T) {
	earlier := now.Add(-time.Hour)
	previous, _ := domain.NewSnapshot(earlier, policy.Interval, []domain.Reading{meterReading(t, "claude", 50, earlier)})
	shelf := &snapshotShelf{previous: previous}
	sources := catalog{"claude": failing(domain.Failure(domain.ErrorUnavailable, "provider down", nil))}

	snapshot, j, err := collect(t, []domain.Subscription{subscription("claude")}, sources, shelf, policy)
	if err != nil {
		t.Fatal(err)
	}
	reading, _ := snapshot.ReadingOf("claude")
	if reading.Outcome() != domain.OutcomeOK || reading.ErrorClass() != domain.ErrorUnavailable || reading.Meters()[0].Used() != 50 {
		t.Fatalf("kept reading = %s/%s", reading.Outcome(), reading.ErrorClass())
	}
	if j.records[0].Freshness != domain.FreshnessStale {
		t.Fatal("an hour-old reading is STALE")
	}
}

func TestCollectSkipsASourceThatAskedToWait(t *testing.T) {
	earlier := now.Add(-time.Minute)
	waiting := domain.Settle(subscription("claude"), ptr(meterReading(t, "claude", 50, earlier)), domain.Reading{}, domain.RateLimited(now.Add(time.Hour), "429"))
	previous, _ := domain.NewSnapshot(earlier, policy.Interval, []domain.Reading{waiting})
	shelf := &snapshotShelf{previous: previous}
	asked := false
	sources := catalog{"claude": sourceFunc(func(context.Context, domain.Subscription) (domain.Reading, error) {
		asked = true
		return domain.Reading{}, nil
	})}

	snapshot, j, err := collect(t, []domain.Subscription{subscription("claude")}, sources, shelf, policy)
	if err != nil {
		t.Fatal(err)
	}
	if asked || j.records[0].Asked {
		t.Fatal("a rate-limited source must not be asked before its retry time")
	}
	if reading, _ := snapshot.ReadingOf("claude"); reading.ErrorClass() != domain.ErrorRateLimited {
		t.Fatalf("kept reading = %s", reading.ErrorClass())
	}
}

func TestCollectGivesUpOnASlowSourceAtTheDeadline(t *testing.T) {
	shelf := &snapshotShelf{readErr: errors.New("corrupt snapshot")}
	release := make(chan struct{})
	defer close(release)
	sources := catalog{
		"claude": sourceFunc(func(context.Context, domain.Subscription) (domain.Reading, error) {
			<-release // ignores its context on purpose
			return domain.Reading{}, nil
		}),
	}
	snapshot, _, err := collect(t, []domain.Subscription{subscription("claude")}, sources, shelf,
		application.CollectionPolicy{Interval: time.Minute, Deadline: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	reading, _ := snapshot.ReadingOf("claude")
	if reading.Outcome() != domain.OutcomeUnavailable || reading.ErrorClass() != domain.ErrorUnavailable {
		t.Fatalf("slow source = %s/%s", reading.Outcome(), reading.ErrorClass())
	}
}

func TestCollectIsolatesAPanickingAdapter(t *testing.T) {
	shelf := &snapshotShelf{readErr: domain.ErrNoSnapshot}
	sources := catalog{
		"claude": sourceFunc(func(context.Context, domain.Subscription) (domain.Reading, error) { panic("bug") }),
		"codex":  answering(meterReading(t, "codex", 10, now)),
	}
	snapshot, _, err := collect(t, []domain.Subscription{subscription("claude"), subscription("codex")}, sources, shelf, policy)
	if err != nil {
		t.Fatal(err)
	}
	if reading, _ := snapshot.ReadingOf("claude"); reading.Outcome() != domain.OutcomeBroken {
		t.Fatalf("panicking adapter = %s", reading.Outcome())
	}
	if reading, _ := snapshot.ReadingOf("codex"); reading.Outcome() != domain.OutcomeOK {
		t.Fatalf("healthy adapter = %s", reading.Outcome())
	}
}

func TestCollectReportsAStoreFailure(t *testing.T) {
	shelf := &snapshotShelf{readErr: domain.ErrNoSnapshot, keepErr: errors.New("disk full")}
	_, j, err := collect(t, []domain.Subscription{subscription("claude")}, catalog{}, shelf, policy)
	if err == nil || len(j.records) != 0 {
		t.Fatalf("Collect = %v, journal %d", err, len(j.records))
	}
}

func TestCollectRefusesDuplicatedSubscriptions(t *testing.T) {
	shelf := &snapshotShelf{readErr: domain.ErrNoSnapshot}
	_, _, err := collect(t, []domain.Subscription{subscription("claude"), subscription("claude")}, catalog{}, shelf, policy)
	if !errors.Is(err, domain.ErrInvalidSnapshot) || shelf.keepCalled != 0 {
		t.Fatalf("Collect = %v", err)
	}
}

func ptr(reading domain.Reading) *domain.Reading { return &reading }
