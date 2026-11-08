//go:build integration

package keycheck_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/keycheck"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func builder(provider keycheck.Provider) providerhttptest.Build {
	return func(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
		return provider.NewSource(endpoint, keys, "checked", func() time.Time { return providerhttptest.Now })
	}
}

var checks = []struct {
	provider     keycheck.Provider
	subscription domain.Subscription
	fixture      string
}{
	{keycheck.Groq, domain.NewSubscription("groq", "groq", "groq-api", domain.FidelityReported, ""), "groq-models.json"},
	{keycheck.NVIDIA, domain.NewSubscription("nvidia", "nvidia", "nvidia-api", domain.FidelityReported, ""), "nvidia-functions.json"},
	{keycheck.Gemini, domain.NewSubscription("gemini", "google", "gemini-api", domain.FidelityReported, ""), "gemini-models.json"},
}

func TestEveryCheckConforms(t *testing.T) {
	for _, c := range checks {
		providerhttptest.Check(t, providerhttptest.Case{
			Subscription: c.subscription, Path: c.provider.Path, OK: string(read(t, c.fixture)), KeyHeader: c.provider.KeyHeader,
		}, builder(c.provider))
	}
}

func TestAWorkingKeyIsUnsupportedWithAHint(t *testing.T) {
	for _, c := range checks {
		provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{c.provider.Path: {Body: string(read(t, c.fixture))}})
		reading, err := builder(c.provider)(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), c.subscription)
		if err != nil {
			t.Fatalf("%s: %v", c.provider.Name, err)
		}
		if reading.Outcome() != domain.OutcomeUnsupported || !strings.Contains(reading.Hint(), "the key works") || len(reading.Meters()) != 0 {
			t.Fatalf("%s: outcome %s, hint %q", c.provider.Name, reading.Outcome(), reading.Hint())
		}
		if strings.Contains(reading.Hint(), providerhttptest.Key) {
			t.Fatalf("%s: the hint carries the key", c.provider.Name)
		}
	}
}

func TestGoogleAnswersAnUnknownKeyWith400(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{
		keycheck.Gemini.Path: {Status: 400, Body: `{"error": {"code": 400, "status": "INVALID_ARGUMENT"}}`},
	})
	_, err := builder(keycheck.Gemini)(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), checks[2].subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorKeyRejected {
		t.Fatalf("400 = %v, want a rejected key", err)
	}
	if received := provider.Received()[0]; received.Header.Get("Authorization") != "" {
		t.Fatal("Google got the key as a Bearer token too")
	}
}
