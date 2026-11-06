//go:build integration

package minimaxapi_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/minimaxapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return minimaxapi.New(endpoint, keys, "minimax", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("minimax", "minimax", "minimax-api", domain.FidelityReported, "")

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, "/v1/token_plan/remains", fixture(t, "remains.json"), build)
}
