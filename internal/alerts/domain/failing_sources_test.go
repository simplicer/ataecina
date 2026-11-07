package domain_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
)

func source(subscription, failure string) domain.WatchedSource {
	return domain.WatchedSource{Subscription: subscription, Failure: failure, Hint: "the hint of " + subscription}
}

func TestASourceFailureIsAnnouncedOnceUntilItRecovers(t *testing.T) {
	state, alerts := domain.NewAlertState(nil).EvaluateSources([]domain.WatchedSource{source("zai", "KEY_MISSING"), source("claude", "")}, now)
	if len(alerts) != 1 || alerts[0] != (domain.SourceAlert{Subscription: "zai", Failure: "KEY_MISSING", Hint: "the hint of zai"}) {
		t.Fatalf("first = %+v", alerts)
	}
	later := now.Add(5 * time.Minute)
	state, alerts = state.EvaluateSources([]domain.WatchedSource{source("zai", "KEY_MISSING")}, later)
	if len(alerts) != 0 {
		t.Fatalf("the same failure is announced once: %+v", alerts)
	}
	if failing := state.Failing(); len(failing) != 1 || failing[0].Since != now {
		t.Fatalf("failing = %+v", failing)
	}
	state, alerts = state.EvaluateSources([]domain.WatchedSource{source("zai", "BROKEN")}, later)
	if len(alerts) != 1 || alerts[0].Failure != "BROKEN" {
		t.Fatalf("another reason is announced: %+v", alerts)
	}
	state, _ = state.EvaluateSources([]domain.WatchedSource{source("zai", "")}, later)
	if len(state.Failing()) != 0 {
		t.Fatalf("a source that recovers is forgotten: %+v", state.Failing())
	}
	if _, alerts = state.EvaluateSources([]domain.WatchedSource{source("zai", "BROKEN")}, later); len(alerts) != 1 {
		t.Fatal("failing again after recovering is announced again")
	}
}

func TestASourceNoLongerWatchedIsForgotten(t *testing.T) {
	state, _ := domain.NewAlertState(nil).EvaluateSources([]domain.WatchedSource{source("zai", "BROKEN"), source("ollama", "BROKEN")}, now)
	if failing := state.Failing(); len(failing) != 2 || failing[0].Subscription != "ollama" {
		t.Fatalf("failing, by subscription = %+v", failing)
	}
	state, _ = state.EvaluateSources([]domain.WatchedSource{source("zai", "BROKEN")}, now)
	if failing := state.Failing(); len(failing) != 1 || failing[0].Subscription != "zai" {
		t.Fatalf("failing = %+v", failing)
	}
}

func TestThresholdAndSourceMemoriesKeepEachOther(t *testing.T) {
	reset := now.Add(time.Hour)
	state, _ := domain.NewAlertState(nil, domain.FailingSource{Subscription: "zai", Failure: "BROKEN", Since: now}).
		Evaluate([]domain.WatchedMeter{meter(91, reset)}, now)
	if len(state.Raised()) != 1 || len(state.Failing()) != 1 {
		t.Fatalf("after thresholds = %+v %+v", state.Raised(), state.Failing())
	}
	state, _ = state.EvaluateSources([]domain.WatchedSource{source("zai", "BROKEN")}, now)
	if len(state.Raised()) != 1 || len(state.Failing()) != 1 {
		t.Fatalf("after sources = %+v %+v", state.Raised(), state.Failing())
	}
}
