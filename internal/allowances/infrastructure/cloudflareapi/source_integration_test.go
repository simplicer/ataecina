//go:build integration

package cloudflareapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cloudflareapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp/providerhttptest"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

const account = "0123456789abcdef0123456789abcdef"

func build(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source {
	return cloudflareapi.New(endpoint, keys, "workers-ai", account, func() time.Time { return providerhttptest.Now })
}

var subscription = domain.NewSubscription("workers-ai", "cloudflare", "cloudflare-api", domain.FidelityReported, "")

func TestTheAdapterConforms(t *testing.T) {
	providerhttptest.Check(t, providerhttptest.Case{
		Subscription: subscription, Method: "POST", Path: "/client/v4/graphql", OK: string(read(t, "neurons.json")),
	}, build)
}

func TestTheDailyAllocationIsReadForTheAccount(t *testing.T) {
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/client/v4/graphql": {Body: string(read(t, "neurons.json"))}})
	reading, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meter := reading.Meters()[0]
	resetsAt, _ := meter.ResetsAt()
	if meter.Window() != domain.DailyWindow() || meter.Used() != 33.505 || !resetsAt.Equal(time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("meter = %+v %v", meter, resetsAt)
	}
	request := provider.Received()[0]
	if request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", request.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(request.Body)
	var sent struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(body, &sent); err != nil || sent.Variables["accountTag"] != account || sent.Variables["date"] != "2026-11-06" {
		t.Fatalf("sent %s (%v)", body, err)
	}
}

func TestARefusedQueryIsARejectedKey(t *testing.T) {
	for _, answer := range []string{
		`{"data":null,"errors":[{"message":"not authorized for that account"}]}`,
		`{"data":{"viewer":{"accounts":[]}},"errors":null}`,
	} {
		provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/client/v4/graphql": {Body: answer}})
		_, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
		var failure *domain.SourceFailure
		if !errors.As(err, &failure) || failure.Class() != domain.ErrorKeyRejected {
			t.Fatalf("%s: %v", answer, err)
		}
	}
	provider := providerhttptest.Serve(t, map[string]providerhttptest.Answer{"/client/v4/graphql": {Body: `{"errors":[{"message":"unknown field totalNeurons"}]}`}})
	_, err := build(provider.Endpoint, providerhttptest.Ring{}).Read(context.Background(), subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorContractChanged {
		t.Fatalf("a schema change = %v", err)
	}
}
