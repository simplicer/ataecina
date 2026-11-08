//go:build integration

package artificialanalysis_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/artificialanalysis"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

const key = "aa-synthetic-not-a-real-key"

func serve(t *testing.T, handler http.HandlerFunc) artificialanalysis.Source {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return artificialanalysis.New(endpoint, func(context.Context) (secret.Key, error) { return secret.New([]byte(key)) })
}

func TestEveryPageIsReadWithTheKey(t *testing.T) {
	var asked []string
	source := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.RequestURI())
		if r.Header.Get("x-api-key") != key || r.Header.Get("Authorization") != "" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("page") == "2" {
			w.Write(read(t, "page2.json"))
			return
		}
		w.Write(read(t, "page1.json"))
	})
	ranking, err := source.Fetch(context.Background())
	if err != nil || ranking.Version != "5" || len(ranking.Scores) != 2 {
		t.Fatalf("Fetch = %+v, %v", ranking, err)
	}
	if len(asked) != 2 || asked[0] != "/api/v2/language/models/free?page=1" {
		t.Fatalf("asked %q", asked)
	}
}

func TestFailuresAreReturned(t *testing.T) {
	rejected := serve(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusUnauthorized) })
	if _, err := rejected.Fetch(context.Background()); !errors.Is(err, artificialanalysis.ErrKeyRejected) {
		t.Fatalf("rejected = %v", err)
	}
	for name, handler := range map[string]http.HandlerFunc{
		"rate limited": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "wait", http.StatusTooManyRequests) },
		"not JSON":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
	} {
		if _, err := serve(t, handler).Fetch(context.Background()); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
	endpoint, _ := httpclient.New(httpclient.Options{AllowPlainHTTP: true}).For("http://127.0.0.1:1")
	noKey := artificialanalysis.New(endpoint, func(context.Context) (secret.Key, error) { return secret.Key{}, errors.New("no key") })
	if _, err := noKey.Fetch(context.Background()); err == nil {
		t.Fatal("a missing key went unnoticed")
	}
}
