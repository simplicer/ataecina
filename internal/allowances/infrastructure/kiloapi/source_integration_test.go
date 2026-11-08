//go:build integration

package kiloapi_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/kiloapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

const balancePath = "/api/profile/balance"

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return kiloapi.New(endpoint, keys, "kilo", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("kilo", "kilo", "kilo-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, balancePath, string(read(t, "balance.json")), build)
}

func TestAUsedUpBalanceIsHinted(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{balancePath: {Body: `{"balance": 0, "isDepleted": true}`}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	if left, _ := balance.Remaining(); left.Amount().Float64() != 0 || reading.Hint() == "" {
		t.Fatalf("left %v, hint %q", left, reading.Hint())
	}
}
