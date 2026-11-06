package codexappserver_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexappserver"
)

var answeredAt = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func fixture(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/rate-limits-read.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheRateLimitsAnswerIsDecoded(t *testing.T) {
	latest, banked, err := codexappserver.DecodeRateLimits(fixture(t), answeredAt)
	if err != nil {
		t.Fatal(err)
	}
	codex := latest["codex"]
	if codex.Plan != "plus" || len(codex.Windows) != 2 || codex.Windows[1].Minutes != 10080 || codex.Windows[1].UsedPercent != 41 {
		t.Fatalf("codex = %+v", codex)
	}
	if !codex.ObservedAt.Equal(answeredAt) || !codex.Windows[0].ResetsAt.Equal(time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("instants = %+v", codex)
	}
	if codex.Credits == nil || codex.Credits.Balance != "7.5" {
		t.Fatalf("credits = %+v", codex.Credits)
	}
	if premium := latest["premium"]; len(premium.Windows) != 1 || !premium.Windows[0].ResetsAt.IsZero() {
		t.Fatalf("premium = %+v", premium)
	}
	if banked == nil || *banked != 2 {
		t.Fatalf("banked = %v", banked)
	}
}

func TestTheSingleSnapshotIsTheFallback(t *testing.T) {
	result := []byte(`{"rateLimits":{"primary":{"usedPercent":3,"windowDurationMins":300}}}`)
	latest, banked, err := codexappserver.DecodeRateLimits(result, answeredAt)
	if err != nil || banked != nil || len(latest["codex"].Windows) != 1 {
		t.Fatalf("DecodeRateLimits = %+v, %v, %v", latest, banked, err)
	}
}

func TestUnexpectedAnswersAreRejected(t *testing.T) {
	for _, result := range []string{
		`[]`,
		`{}`,
		`{"rateLimits":{"primary":{"usedPercent":"x","windowDurationMins":300}}}`,
		`{"rateLimits":{"primary":{"usedPercent":3}}}`,
		`{"rateLimits":{"primary":{"usedPercent":-3,"windowDurationMins":300}}}`,
		`{"rateLimits":{"primary":{"usedPercent":3,"windowDurationMins":300}},"rateLimitResetCredits":{"availableCount":-1}}`,
	} {
		if _, _, err := codexappserver.DecodeRateLimits([]byte(result), answeredAt); err == nil {
			t.Fatalf("DecodeRateLimits(%s) accepted", result)
		}
	}
}

// FuzzDecodeRateLimits checks that no answer panics the decoder or yields
// impossible windows.
func FuzzDecodeRateLimits(f *testing.F) {
	f.Add(fixture(f))
	f.Add([]byte(`{"rateLimits":{"primary":{"usedPercent":1e308,"windowDurationMins":1,"resetsAt":-5}}}`))
	f.Add([]byte(`{"rateLimitsByLimitId":{"x":{}}}`))
	f.Fuzz(func(t *testing.T, result []byte) {
		latest, banked, err := codexappserver.DecodeRateLimits(result, answeredAt)
		if err != nil {
			return
		}
		if banked != nil && *banked < 0 {
			t.Fatalf("negative banked resets from %q", result)
		}
		for _, limits := range latest {
			for _, w := range limits.Windows {
				if w.Minutes <= 0 || w.UsedPercent < 0 {
					t.Fatalf("impossible window %+v from %q", w, result)
				}
			}
		}
	})
}
