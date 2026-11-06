//go:build integration

package orcarouterapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/orcarouterapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return orcarouterapi.New(endpoint, keys, "orca", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("orca", "orcarouter", "orcarouter-api", domain.FidelityReported, "")

func answers(t *testing.T, subscriptionBody string) map[string]providerhttptest.Answer {
	return map[string]providerhttptest.Answer{
		"/v1/dashboard/billing/subscription": {Body: subscriptionBody},
		"/v1/dashboard/billing/usage":        {Body: string(read(t, "usage.json"))},
	}
}

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Check(t, providerhttptest.Case{
		Subscription: subscription,
		Path:         "/v1/dashboard/billing/subscription",
		OK:           string(read(t, "subscription.json")),
		Also:         map[string]providerhttptest.Answer{"/v1/dashboard/billing/usage": {Body: string(read(t, "usage.json"))}},
	}, build)
}

func TestWhatIsLeftOfTheAllowance(t *testing.T) {
	provider := providerhttptest.Serve(t, answers(t, string(read(t, "subscription.json"))))
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	remaining, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if remaining.Amount().String() != "8.42" || spent.Amount().String() != "11.58" || reading.Hint() != "the allowance expires on 2026-12-31" {
		t.Fatalf("balance = %v left, %v spent, hint %q", remaining, spent, reading.Hint())
	}
}

func TestAnUnlimitedKeyShowsOnlyTheSpending(t *testing.T) {
	provider := providerhttptest.Serve(t, answers(t, `{"hard_limit_usd": 100000000, "access_until": 0}`))
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	if _, ok := balance.Remaining(); ok || reading.Hint() != "the key has no spending limit" {
		t.Fatalf("unlimited key = %+v", reading)
	}
}

func TestAFailingUsageSummaryFailsTheReading(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{
		"/v1/dashboard/billing/subscription": {Body: string(read(t, "subscription.json"))},
		"/v1/dashboard/billing/usage":        {Status: 503},
	})
	_, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnavailable {
		t.Fatalf("Read = %v", err)
	}
}
