//go:build integration

package moonshotapi_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/moonshotapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

const balancePath = "/v1/users/me/balance"

func on(platform moonshotapi.Platform) providerhttptest.Build {
	return func(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
		return moonshotapi.New(platform)(endpoint, keys, "moonshot", func() time.Time { return providerhttptest.Now })
	}
}

var subscription = domain.NewSubscription("moonshot", "moonshot", "moonshot-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, balancePath, string(read(t, "balance.json")), on(moonshotapi.Global))
}

func TestEachPlatformBillsInItsCurrency(t *testing.T) {
	for platform, currency := range map[moonshotapi.Platform]string{moonshotapi.Global: "USD", moonshotapi.China: "CNY"} {
		provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{balancePath: {Body: string(read(t, "balance.json"))}})
		reading, err := on(platform)(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
		if err != nil {
			t.Fatal(err)
		}
		balance, _ := reading.Balance()
		left, _ := balance.Remaining()
		if left.Amount().Float64() != 12.75 || left.Currency().String() != currency || reading.Hint() != "" {
			t.Fatalf("%s: left %v, hint %q", currency, left, reading.Hint())
		}
	}
}

func TestMoneyOwedIsHinted(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{balancePath: {
		Body: `{"code":0,"status":true,"data":{"available_balance":-0.5,"voucher_balance":0,"cash_balance":-0.5}}`,
	}})
	reading, err := on(moonshotapi.Global)(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil || reading.Hint() == "" {
		t.Fatalf("owed = %q, %v", reading.Hint(), err)
	}
}
