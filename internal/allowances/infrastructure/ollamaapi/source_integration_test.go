//go:build integration

package ollamaapi_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/ollamaapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return ollamaapi.New(endpoint, keys, "ollama", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("ollama", "ollama", "ollama-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, "/api/usage", string(read(t, "usage.json")), build)
}

func TestTheWindowsAreReadWithoutAReset(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/api/usage": {Body: string(read(t, "usage.json"))}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if len(meters) != 2 || meters[0].Window() != domain.FiveHourWindow() || meters[0].Used() != 23.5 || meters[1].Window() != domain.WeeklyWindow() {
		t.Fatalf("meters = %+v", meters)
	}
	if _, ok := meters[0].ResetsAt(); ok {
		t.Fatal("Ollama gives no reset instant")
	}
}
