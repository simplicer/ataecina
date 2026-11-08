//go:build integration

package xaiapi_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/xaiapi"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

const balancePath = "/v1/billing/teams/team-synthetic/prepaid/balance"

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return xaiapi.New(endpoint, keys, "xai", "team-synthetic", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("xai", "xai", "xai-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, balancePath, string(read(t, "balance.json")), build)
}

func TestTheTeamCreditIsTheMoneyLeft(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{balancePath: {Body: string(read(t, "balance.json"))}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	balance, _ := reading.Balance()
	if left, _ := balance.Remaining(); left.Amount().Float64() != 17.5 {
		t.Fatalf("left = %v", left)
	}
}

func TestTheTeamIdStaysInThePath(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/v1/billing/teams/a%2Fb/prepaid/balance": {Body: string(read(t, "balance.json"))}})
	source := xaiapi.New(provider.Endpoint, providerhttptest.Ring{}, "xai", "a/b", func() time.Time { return providerhttptest.Now })
	if _, err := source.Read(context.Background(), subscription); err != nil {
		t.Fatalf("an escaped team id = %v", err)
	}
}
