//go:build integration

package commandcodeapi_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/commandcodeapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return commandcodeapi.New(endpoint, keys, "commandcode", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("commandcode", "commandcode", "commandcode-api", domain.FidelityReported, "")

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Conformance(t, subscription, "/alpha/billing/credits", fixture(t, "credits.json"), build)
}
