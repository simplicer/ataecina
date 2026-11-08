//go:build integration

package kimicodeapi_test

import (
	"context"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/kimicodeapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

const usagesPath = "/coding/v1/usages"

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return kimicodeapi.New(endpoint, keys, "kimi", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("kimi", "kimi", "kimi-code-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, usagesPath, string(read(t, "usages.json")), build)
}

func TestTheMembershipIsAPlanWithTwoWindows(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{usagesPath: {Body: string(read(t, "usages.json"))}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if reading.Plan() != "INTERMEDIATE" || len(meters) != 2 || meters[0].Used() != 12 || meters[1].Window() != domain.WeeklyWindow() {
		t.Fatalf("plan %q, meters %+v", reading.Plan(), meters)
	}
}
