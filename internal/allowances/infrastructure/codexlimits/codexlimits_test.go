package codexlimits_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
)

var observed = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

func codex() domain.Subscription {
	return domain.NewSubscription("codex", "openai", "codex-logs", domain.FidelityReported, "")
}

func TestEveryBucketWithWindowsBecomesMeters(t *testing.T) {
	banked := 2
	latest := map[string]codexlimits.Limits{
		"codex": {ObservedAt: observed, LimitID: "codex", Plan: "plus",
			Windows: []codexlimits.Window{
				{Minutes: 10080, UsedPercent: 40, ResetsAt: observed.Add(72 * time.Hour)},
				{Minutes: 300, UsedPercent: 20, ResetsAt: observed.Add(time.Hour)},
				{Minutes: 300, UsedPercent: 99},
			},
			Credits: &codexlimits.Credits{HasCredits: true, Balance: "4.25"}},
		"premium":              {ObservedAt: observed.Add(time.Minute), LimitID: "premium", Windows: []codexlimits.Window{{Minutes: 10080, UsedPercent: 120}}},
		"base_model_inference": {ObservedAt: observed, LimitID: "base_model_inference"},
	}
	reading, err := codexlimits.Reading(codex(), "codex-app-server", latest, &banked)
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if len(meters) != 3 {
		t.Fatalf("%d meters", len(meters))
	}
	if meters[0].Window() != domain.WeeklyWindow() || meters[0].Bucket() != "codex" || meters[1].Window() != domain.FiveHourWindow() || meters[1].Used() != 20 {
		t.Fatalf("codex meters = %+v", meters[:2])
	}
	if meters[2].Bucket() != "premium" || meters[2].Used() != 120 {
		t.Fatalf("premium meter = %+v", meters[2])
	}
	if credits, ok := reading.Credits(); !ok || credits.Available().String() != "4.25" {
		t.Fatalf("credits = %+v", credits)
	}
	if resets, ok := reading.BankedResets(); !ok || resets.Available() != 2 {
		t.Fatal("banked resets are informative and kept")
	}
	if reading.Plan() != "PLUS" || reading.Source() != "codex-app-server" {
		t.Fatalf("plan %q source %q", reading.Plan(), reading.Source())
	}
	if at, _ := reading.ObservedAt(); !at.Equal(observed.Add(time.Minute)) {
		t.Fatalf("observed at %v", at)
	}
}

func TestCreditsWithoutBalanceOrUnlimited(t *testing.T) {
	latest := map[string]codexlimits.Limits{"codex": {ObservedAt: observed, LimitID: "codex",
		Windows: []codexlimits.Window{{Minutes: 300, UsedPercent: 1}},
		Credits: &codexlimits.Credits{Unlimited: true}}}
	reading, err := codexlimits.Reading(codex(), "codex-logs", latest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if credits, _ := reading.Credits(); !credits.Unlimited() || credits.Available().String() != "0" {
		t.Fatalf("credits = %+v", credits)
	}
	latest["codex"] = codexlimits.Limits{ObservedAt: observed, LimitID: "codex",
		Windows: []codexlimits.Window{{Minutes: 300, UsedPercent: 1}},
		Credits: &codexlimits.Credits{HasCredits: false, Balance: "0"}}
	reading, _ = codexlimits.Reading(codex(), "codex-logs", latest, nil)
	if _, ok := reading.Credits(); ok {
		t.Fatal("a plan without credits shows none")
	}
}

func TestWindowsAreValidated(t *testing.T) {
	window, err := codexlimits.NewWindow(300, 120, time.Time{})
	if err != nil || window.Minutes != 300 || !window.ResetsAt.IsZero() {
		t.Fatalf("NewWindow = %+v, %v", window, err)
	}
	for _, bad := range []struct {
		minutes int
		used    float64
	}{{0, 1}, {-5, 1}, {300, -1}, {300, math.NaN()}, {300, math.Inf(1)}} {
		if _, err := codexlimits.NewWindow(bad.minutes, bad.used, time.Time{}); err == nil {
			t.Fatalf("NewWindow(%d, %v) accepted", bad.minutes, bad.used)
		}
	}
}

func TestNothingToShow(t *testing.T) {
	latest := map[string]codexlimits.Limits{"codex": {ObservedAt: observed, LimitID: "codex"}}
	if _, err := codexlimits.Reading(codex(), "codex-logs", latest, nil); !errors.Is(err, codexlimits.ErrNoFigures) {
		t.Fatalf("Reading = %v", err)
	}
}

func TestOutOfRangeFiguresAreErrors(t *testing.T) {
	cases := []codexlimits.Limits{
		{ObservedAt: observed, LimitID: "codex", Windows: []codexlimits.Window{{Minutes: 0, UsedPercent: 1}}},
		{ObservedAt: observed, LimitID: "codex", Windows: []codexlimits.Window{{Minutes: 300, UsedPercent: -1}}},
		{ObservedAt: observed, LimitID: "codex", Windows: []codexlimits.Window{{Minutes: 300, UsedPercent: 1}}, Credits: &codexlimits.Credits{HasCredits: true, Balance: "lots"}},
	}
	for _, limits := range cases {
		if _, err := codexlimits.Reading(codex(), "codex-logs", map[string]codexlimits.Limits{"codex": limits}, nil); err == nil {
			t.Fatalf("Reading(%+v) accepted", limits)
		}
	}
	negative := -1
	ok := map[string]codexlimits.Limits{"codex": {ObservedAt: observed, LimitID: "codex", Windows: []codexlimits.Window{{Minutes: 300, UsedPercent: 1}}}}
	if _, err := codexlimits.Reading(codex(), "codex-logs", ok, &negative); err == nil {
		t.Fatal("negative banked resets accepted")
	}
}
