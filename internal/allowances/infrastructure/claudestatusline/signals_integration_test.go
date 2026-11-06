//go:build integration

package claudestatusline_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/claudestatusline"
)

var observed = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func signalReading(t *testing.T) domain.Reading {
	t.Helper()
	meters, err := claudestatusline.DecodeRateLimits(fixture(t, "statusline-pro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: claudestatusline.SourceID,
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: observed, Meters: meters,
	})
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

var claude = domain.NewSubscription("claude", "anthropic", claudestatusline.SourceID, domain.FidelityReported, "")

func TestTheSignalIsKeptPrivatelyAndReadByTheCollector(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run", "ataecina", "signals")
	signals := claudestatusline.NewSignals(dir)
	if _, err := signals.LatestSignal("claude"); !errors.Is(err, domain.ErrNoSignal) {
		t.Fatalf("before the hook: %v", err)
	}
	if err := signals.KeepSignal(signalReading(t)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "claude.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("signal file = %v", err)
	}
	reading, err := claudestatusline.NewSource(dir).Read(context.Background(), claude)
	if err != nil || len(reading.Meters()) != 2 || reading.Meters()[1].Used() != 41.2 {
		t.Fatalf("collector read = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "claude.json"))
	for _, leaked := range []string{"transcript_path", "/home/someone", "session_id", "cost"} {
		if bytes.Contains(data, []byte(leaked)) {
			t.Fatalf("the signal keeps %q from the status line input", leaked)
		}
	}
}

func TestTheCollectorExplainsAMissingOrBrokenSignal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "signals")
	_, err := claudestatusline.NewSource(dir).Read(context.Background(), claude)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("no signal = %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude.json"), []byte(`{"version":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := claudestatusline.NewSource(dir).Read(context.Background(), claude); !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("broken signal = %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := claudestatusline.NewSignals(dir).KeepSignal(signalReading(t)); err == nil {
		t.Fatal("a loose signals directory must be refused")
	}
}
