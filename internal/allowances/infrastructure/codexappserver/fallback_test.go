package codexappserver_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexappserver"
)

type stubSource struct {
	reading domain.Reading
	err     error
	asked   *int
}

func (s stubSource) Read(context.Context, domain.Subscription) (domain.Reading, error) {
	if s.asked != nil {
		*s.asked++
	}
	return s.reading, s.err
}

func codexReading(t *testing.T, source domain.SourceID, observedAt time.Time) domain.Reading {
	t.Helper()
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "codex", Provider: "openai", Source: source, Fidelity: domain.FidelityReported,
		Outcome: domain.OutcomeOK, ObservedAt: observedAt,
		Meters: []domain.Meter{domain.NewMeter("codex", domain.FiveHourWindow(), 1, time.Time{})},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

func readFallback(logs, server stubSource) (domain.Reading, error) {
	fallback := codexappserver.NewFallback(logs, server, time.Hour, func() time.Time { return answeredAt })
	return fallback.Read(context.Background(), domain.NewSubscription("codex", "openai", "codex-logs", domain.FidelityReported, ""))
}

func TestFreshLogsNeedNoServer(t *testing.T) {
	asked := 0
	reading, err := readFallback(
		stubSource{reading: codexReading(t, "codex-logs", answeredAt.Add(-10*time.Minute))},
		stubSource{asked: &asked},
	)
	if err != nil || reading.Source() != "codex-logs" || asked != 0 {
		t.Fatalf("Read = %s, %v, server asked %d times", reading.Source(), err, asked)
	}
}

func TestStaleLogsAskTheServer(t *testing.T) {
	reading, err := readFallback(
		stubSource{reading: codexReading(t, "codex-logs", answeredAt.Add(-2*time.Hour))},
		stubSource{reading: codexReading(t, "codex-app-server", answeredAt)},
	)
	if err != nil || reading.Source() != "codex-app-server" {
		t.Fatalf("Read = %s, %v", reading.Source(), err)
	}
}

func TestTheLogsRemainTheSourceWhenTheServerFails(t *testing.T) {
	down := domain.Failure(domain.ErrorUnavailable, "down", nil)
	reading, err := readFallback(
		stubSource{reading: codexReading(t, "codex-logs", answeredAt.Add(-2*time.Hour))},
		stubSource{err: down},
	)
	if err != nil || reading.Source() != "codex-logs" {
		t.Fatalf("Read = %s, %v", reading.Source(), err)
	}
	noLogs := domain.Failure(domain.ErrorUnreadable, "no logs", nil)
	if _, err := readFallback(stubSource{err: noLogs}, stubSource{err: down}); err != noLogs {
		t.Fatalf("Read = %v, want the logs' failure", err)
	}
	reading, err = readFallback(stubSource{err: noLogs}, stubSource{reading: codexReading(t, "codex-app-server", answeredAt)})
	if err != nil || reading.Source() != "codex-app-server" {
		t.Fatalf("Read = %s, %v", reading.Source(), err)
	}
}
