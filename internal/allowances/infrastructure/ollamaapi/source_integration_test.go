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

const balancePath = "/api/balance"

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return ollamaapi.New(endpoint, keys, "ollama", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("ollama", "ollama", "ollama-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, balancePath, string(read(t, "balance.json")), build)
}

func readAnswer(t *testing.T, body string) domain.Reading {
	t.Helper()
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{balancePath: {Body: body}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

func TestTheIncludedCreditIsAMeterAndMoneyLeft(t *testing.T) {
	reading := readAnswer(t, string(read(t, "balance.json")))
	meters := reading.Meters()
	if len(meters) != 1 || meters[0].Window() != domain.BillingCycleWindow() || meters[0].Used() != 25 {
		t.Fatalf("meters = %+v", meters)
	}
	if resetsAt, _ := meters[0].ResetsAt(); !resetsAt.Equal(time.Date(2026, 11, 20, 8, 15, 0, 0, time.UTC)) {
		t.Fatalf("reborn at %s", resetsAt)
	}
	balance, _ := reading.Balance()
	left, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if left.Amount().Float64() != 50.5 || spent.Amount().Float64() != 15 {
		t.Fatalf("left %v, spent %v", left, spent)
	}
}

func TestALegacyPlanShowsItsWindows(t *testing.T) {
	reading := readAnswer(t, string(read(t, "legacy.json")))
	meters := reading.Meters()
	if len(meters) != 2 || meters[0].Window() != domain.FiveHourWindow() || meters[0].Used() != 23.5 || meters[1].Window() != domain.WeeklyWindow() || meters[1].Used() != 41 {
		t.Fatalf("meters = %+v", meters)
	}
	if _, ok := reading.Balance(); ok {
		t.Fatal("money without a credit")
	}
}

func TestPayAsYouGoShowsThePurchasedCredit(t *testing.T) {
	reading := readAnswer(t, `{"purchased": {"balance_usd": 3}}`)
	balance, _ := reading.Balance()
	if left, _ := balance.Remaining(); left.Amount().Float64() != 3 || len(reading.Meters()) != 0 {
		t.Fatalf("left %v, meters %+v", left, reading.Meters())
	}
}
