package terminal_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
)

func TestEachSubscriptionFitsOnePlainLine(t *testing.T) {
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, sample(t))
	if err != nil {
		t.Fatal(err)
	}
	report := snapshot.StatusAt(taken)
	want := map[string]string{
		"claude":  "claude · PRO · 5h 23% ↻ 2h10m · week 41.3% ↻ 4d12h · fresh 2m ago",
		"zen":     "zen · ≈ $3.10 left · $16.90 spent · stale 3d ago · derived",
		"minimax": "minimax · broken: unexpected shape",
	}
	for _, status := range report.Statuses() {
		line, ok := want[status.Reading().Subscription().String()]
		if !ok {
			continue
		}
		if got := terminal.Line(status, taken); got != line {
			t.Fatalf("Line = %q\nwant   %q", got, line)
		}
	}
	if !terminal.Attention(report) {
		t.Fatal("codex at 96 % needs attention")
	}
	if got := terminal.Updated(taken, taken.Add(2*time.Minute)); got != "updated 2m ago" {
		t.Fatalf("Updated = %q", got)
	}
}

func TestNoAttentionBelowTheWarningLevel(t *testing.T) {
	calm := mustReading(t, domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), 89.9, taken.Add(time.Hour))},
	})
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, []domain.Reading{calm})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Attention(snapshot.StatusAt(taken)) || terminal.Attention(domain.StatusReport{}) {
		t.Fatal("no attention below 90 % or without a snapshot")
	}
}
