//go:build integration

package cursoradminapi_test

import (
	"context"
	"encoding/base64"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cursoradminapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return cursoradminapi.New(endpoint, keys, "cursor", "member@example.com", func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("cursor", "cursor", "cursor-admin-api", domain.FidelityReported, "")

func TestTheMembersOnDemandLimitIsAMeter(t *testing.T) {
	var authorization, method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization, method = r.Header.Get("Authorization"), r.Method
		w.Write(read(t, "spend.json"))
	}))
	defer server.Close()
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	reading, err := build(endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || authorization != "Basic "+base64.StdEncoding.EncodeToString([]byte(providerhttptest.Key+":")) {
		t.Fatalf("request = %s %q", method, authorization)
	}
	meters := reading.Meters()
	if len(meters) != 1 || math.Abs(meters[0].Used().Float64()-25.01) > 1e-9 || meters[0].Window() != domain.BillingCycleWindow() {
		t.Fatalf("meters = %+v", meters)
	}
	balance, _ := reading.Balance()
	left, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if left.Amount().Float64() != 37.495 || spent.Amount().Float64() != 42.505 {
		t.Fatalf("left %v, spent %v", left, spent)
	}
}
