//go:build integration

package publishedoffers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/publishedoffers"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func serve(t *testing.T, handler http.HandlerFunc) publishedoffers.Source {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return publishedoffers.New(endpoint)
}

func TestThePublishedFileIsFetched(t *testing.T) {
	var path, authorization string
	source := serve(t, func(w http.ResponseWriter, r *http.Request) {
		path, authorization = r.URL.Path, r.Header.Get("Authorization")
		w.Write(read(t, "offers.csv"))
	})
	offers, err := source.Fetch(context.Background())
	if err != nil || len(offers) != 2 || path != publishedoffers.Path || authorization != "" {
		t.Fatalf("Fetch = %+v, %v (%s %q)", offers, err, path, authorization)
	}
}

func TestNothingPublishedIsNoOffer(t *testing.T) {
	offers, err := serve(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }).Fetch(context.Background())
	if err != nil || offers == nil || len(offers) != 0 {
		t.Fatalf("not published = %+v, %v", offers, err)
	}
	for name, handler := range map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) },
		"not CSV":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
	} {
		if _, err := serve(t, handler).Fetch(context.Background()); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
}
