//go:build integration

package openrouterbenchmarks_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/openrouterbenchmarks"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

const key = "sk-or-synthetic-not-a-real-key"

func serve(t *testing.T, handler http.HandlerFunc) openrouterbenchmarks.Source {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return openrouterbenchmarks.New(endpoint, func(context.Context) (secret.Key, error) { return secret.New([]byte(key)) })
}

func TestTheBenchmarksAreReadWithTheOpenRouterKey(t *testing.T) {
	var asked, authorization string
	source := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked, authorization = r.URL.RequestURI(), r.Header.Get("Authorization")
		w.Write(read(t, "benchmarks.json"))
	})
	ranking, err := source.Fetch(context.Background())
	if err != nil || ranking.Via != "OpenRouter" || len(ranking.Scores) != 2 {
		t.Fatalf("Fetch = %+v, %v", ranking, err)
	}
	if asked != "/api/v1/benchmarks?source=artificial-analysis" || authorization != "Bearer "+key {
		t.Fatalf("request = %s %q", asked, authorization)
	}
}

func TestFailuresAreReturned(t *testing.T) {
	rejected := serve(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusUnauthorized) })
	if _, err := rejected.Fetch(context.Background()); !errors.Is(err, openrouterbenchmarks.ErrKeyRejected) {
		t.Fatalf("rejected = %v", err)
	}
	for name, handler := range map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) },
		"not JSON":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
	} {
		if _, err := serve(t, handler).Fetch(context.Background()); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
	endpoint, _ := httpclient.New(httpclient.Options{AllowPlainHTTP: true}).For("http://127.0.0.1:1")
	noKey := openrouterbenchmarks.New(endpoint, func(context.Context) (secret.Key, error) { return secret.Key{}, errors.New("no key") })
	if _, err := noKey.Fetch(context.Background()); err == nil {
		t.Fatal("a missing key went unnoticed")
	}
}
