// Package providerhttptest serves recorded, synthetic provider answers on
// a local httptest server and checks that a source adapter handles every
// class of failure the same way. For tests only.
package providerhttptest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Key is the synthetic key every test seals; it never matches a real one.
const Key = "sk-ataecina-test-not-a-real-key"

// Now is the instant of every test collection.
var Now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

// Answer is what the fake provider replies on one path.
type Answer struct {
	Status int
	Body   string
	Header http.Header
}

// Provider is a fake provider and what it received.
type Provider struct {
	mu       sync.Mutex
	answers  map[string]Answer
	received []*http.Request
	Endpoint httpclient.Endpoint
}

// Serve starts a fake provider answering per path (with its query).
func Serve(t testing.TB, answers map[string]Answer) *Provider {
	t.Helper()
	p := &Provider{answers: answers}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep the body so that tests can read what was sent.
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		received := r.Clone(context.Background())
		received.Body = io.NopCloser(bytes.NewReader(body))
		p.mu.Lock()
		p.received = append(p.received, received)
		answer, ok := p.answers[r.URL.RequestURI()]
		p.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		for name, values := range answer.Header {
			w.Header()[name] = values
		}
		status := answer.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer.Body))
	}))
	t.Cleanup(server.Close)
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	p.Endpoint = endpoint
	return p
}

// Received lists the requests the provider got.
func (p *Provider) Received() []*http.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*http.Request(nil), p.received...)
}

// Ring is a KeyRing holding the synthetic key, or failing.
type Ring struct{ Failure error }

// KeyFor returns Key or the configured failure.
func (r Ring) KeyFor(context.Context, domain.KeyName) (secret.Key, error) {
	if r.Failure != nil {
		return secret.Key{}, r.Failure
	}
	return secret.New([]byte(Key))
}

// Build makes the adapter under test from an endpoint and a key ring.
type Build func(endpoint httpclient.Endpoint, keys domain.KeyRing) domain.Source

// Case is what an adapter is checked with: the request on Path (by Method,
// GET when empty) whose failures are classified, its good answer, and the
// good answers of any other path the adapter also needs.
type Case struct {
	Subscription domain.Subscription
	Method       string
	Path         string
	OK           string
	Also         map[string]Answer
}

// Conformance checks one GET path; see Check.
func Conformance(t *testing.T, subscription domain.Subscription, path, okBody string, build Build) {
	t.Helper()
	Check(t, Case{Subscription: subscription, Path: path, OK: okBody}, build)
}

// Check verifies the request an adapter sends on c.Path and its handling
// of each failure class (SPECS §4.3).
func Check(t *testing.T, c Case, build Build) {
	t.Helper()
	method := c.Method
	if method == "" {
		method = http.MethodGet
	}
	subscription, path, okBody := c.Subscription, c.Path, c.OK
	read := func(answer Answer, keys domain.KeyRing) (*Provider, domain.Reading, error) {
		answers := map[string]Answer{path: answer}
		for other, good := range c.Also {
			answers[other] = good
		}
		provider := Serve(t, answers)
		reading, err := build(provider.Endpoint, keys).Read(context.Background(), subscription)
		return provider, reading, err
	}

	provider, reading, err := read(Answer{Body: okBody}, Ring{})
	if err != nil {
		t.Fatalf("a good answer = %v", err)
	}
	if reading.Subscription() != subscription.ID() {
		t.Fatalf("reading for %s", reading.Subscription())
	}
	first := provider.Received()[0]
	if first.Header.Get("Authorization") != "Bearer "+Key || first.Header.Get("User-Agent") != "ataecina/test" || first.Method != method {
		t.Fatalf("request = %s %v", first.Method, first.Header)
	}

	failures := []struct {
		name   string
		answer Answer
		class  domain.ErrorClass
	}{
		{"401", Answer{Status: 401, Body: `{"error":"bad key"}`}, domain.ErrorKeyRejected},
		{"403", Answer{Status: 403}, domain.ErrorKeyRejected},
		{"429", Answer{Status: 429, Header: http.Header{"Retry-After": {"120"}}}, domain.ErrorRateLimited},
		{"429 without Retry-After", Answer{Status: 429}, domain.ErrorRateLimited},
		{"503", Answer{Status: 503}, domain.ErrorUnavailable},
		{"404", Answer{Status: 404}, domain.ErrorContractChanged},
		{"not JSON", Answer{Body: "<html>"}, domain.ErrorContractChanged},
		{"empty object", Answer{Body: "{}"}, domain.ErrorContractChanged},
	}
	for _, f := range failures {
		_, _, err := read(f.answer, Ring{})
		var failure *domain.SourceFailure
		if !errors.As(err, &failure) || failure.Class() != f.class {
			t.Fatalf("%s: %v, want %s", f.name, err, f.class)
		}
		if strings.Contains(failure.Error(), Key) {
			t.Fatalf("%s: the failure carries the key", f.name)
		}
	}
	_, _, err = read(Answer{Status: 429, Header: http.Header{"Retry-After": {"120"}}}, Ring{})
	var limited *domain.SourceFailure
	if errors.As(err, &limited) {
		if retryAt, ok := limited.RetryAt(); !ok || retryAt.Before(Now.Add(100*time.Second)) {
			t.Fatalf("Retry-After ignored: %v", retryAt)
		}
	}
	missing := domain.KeyMissing("set it")
	if _, _, err := read(Answer{Body: okBody}, Ring{Failure: missing}); !errors.Is(err, missing) {
		t.Fatalf("a missing key = %v", err)
	}
}
