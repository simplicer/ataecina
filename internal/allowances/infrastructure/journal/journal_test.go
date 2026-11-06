package journal_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/journal"
)

func TestOneLokiLinePerSubscription(t *testing.T) {
	var out bytes.Buffer
	log := journal.New(slog.New(slog.NewJSONHandler(&out, nil)))
	reading, _ := domain.NewReading(domain.ReadingFacts{
		Subscription: "zai", Provider: "zai", Source: "zai-api", Fidelity: domain.FidelityReported,
		Outcome: domain.OutcomeBroken, ErrorClass: domain.ErrorContractChanged,
	})
	log.Record(domain.CollectionRecord{Reading: reading, Freshness: domain.FreshnessUnknown, Latency: 42 * time.Millisecond, Asked: true})

	var line map[string]any
	if err := json.Unmarshal(out.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"msg": "subscription collected", "subscription": "zai", "source": "zai-api", "fidelity": "REPORTED",
		"outcome": "BROKEN", "error_class": "CONTRACT_CHANGED", "latency_ms": 42.0, "freshness": "UNKNOWN", "asked": true,
	}
	for key, value := range want {
		if line[key] != value {
			t.Fatalf("%s = %v, want %v", key, line[key], value)
		}
	}
}
