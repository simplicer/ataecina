//go:build integration

package heimdallplans_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/heimdallplans"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return heimdallplans.New(endpoint, keys, "heimdall", func() time.Time { return providerhttptest.Now })
}

func subscriptionOf(provider domain.ProviderID) domain.Subscription {
	return domain.NewSubscription(domain.SubscriptionID(provider), provider, "heimdall", domain.FidelityObserved, "")
}

func readWith(t *testing.T, body string, provider domain.ProviderID) (domain.Reading, error) {
	t.Helper()
	server := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/v1/plans": {Body: body}})
	return build(server.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscriptionOf(provider))
}

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscriptionOf("groq"), "/v1/plans", string(read(t, "plans.json")), build)
}

func TestOnePlanIsTheMainBucket(t *testing.T) {
	reading, err := readWith(t, string(read(t, "plans.json")), "groq")
	if err != nil {
		t.Fatal(err)
	}
	meter := reading.Meters()[0]
	resetsAt, _ := meter.ResetsAt()
	if len(reading.Meters()) != 1 || !meter.Bucket().IsMain() || meter.Used() != 10 || meter.Window() != domain.BillingCycleWindow() ||
		!resetsAt.Equal(time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)) || reading.Fidelity() != domain.FidelityObserved {
		t.Fatalf("groq = %+v", reading.Meters())
	}
}

func TestSeveralPlansAreBucketsAndAnExhaustedOneIsFull(t *testing.T) {
	reading, err := readWith(t, string(read(t, "plans.json")), "google")
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if len(meters) != 2 || meters[0].Bucket().String() != "google-free-a" || meters[0].Used() != 62 || meters[1].Used() != 100 {
		t.Fatalf("google = %+v", meters)
	}
	if reading.Hint() != "google-free-b: quota exhausted" {
		t.Fatalf("hint = %q", reading.Hint())
	}
}

func TestWithoutUsageOrPlansNothingIsMadeUp(t *testing.T) {
	_, err := readWith(t, string(read(t, "plans-without-usage.json")), "google")
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnavailable ||
		!strings.Contains(failure.Error(), "google-free-a: next reset 2026-11-07T08:00:00Z") || !strings.Contains(failure.Error(), "no window usage") {
		t.Fatalf("no usage = %v", err)
	}
	_, err = readWith(t, string(read(t, "plans.json")), "nvidia")
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnavailable || !strings.Contains(failure.Error(), `no plan of provider "nvidia"`) {
		t.Fatalf("no plan = %v", err)
	}
}
