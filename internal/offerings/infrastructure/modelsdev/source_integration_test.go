//go:build integration

package modelsdev_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/modelsdev"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func serve(t *testing.T, handler http.HandlerFunc) modelsdev.Source {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := httpclient.New(httpclient.Options{Version: "test", AllowPlainHTTP: true}).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return modelsdev.New(endpoint)
}

func TestTheCatalogueIsFetchedAndRevalidated(t *testing.T) {
	body := read(t, "api.json")
	var asked []string
	source := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+" "+r.Header.Get("If-None-Match")+" "+r.Header.Get("Authorization"))
		w.Header().Set("ETag", `W/"v1"`)
		if r.Header.Get("If-None-Match") == `W/"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write(body)
	})
	fetched, err := source.Fetch(context.Background(), []string{"openrouter"}, "")
	if err != nil || fetched.NotModified || fetched.Validator != `W/"v1"` || len(fetched.Providers) != 1 {
		t.Fatalf("Fetch = %+v, %v", fetched, err)
	}
	again, err := source.Fetch(context.Background(), []string{"openrouter"}, `W/"v1"`)
	if err != nil || !again.NotModified {
		t.Fatalf("revalidated = %+v, %v", again, err)
	}
	if asked[0] != "/api.json  " || asked[1] != `/api.json W/"v1" ` {
		t.Fatalf("requests = %q (no key is ever sent)", asked)
	}
}

func TestACatalogueLargerThanAProviderAnswerIsAccepted(t *testing.T) {
	padding := make([]byte, 2*httpclient.MaxBodyBytes)
	for i := range padding {
		padding[i] = ' '
	}
	body := append(padding, read(t, "api.json")...)
	source := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write(body) })
	if fetched, err := source.Fetch(context.Background(), []string{"zai-coding-plan"}, ""); err != nil || len(fetched.Providers) != 1 {
		t.Fatalf("Fetch = %+v, %v", fetched, err)
	}
}

func TestFailuresAreReturned(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) },
		"not JSON":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://elsewhere.example/", http.StatusFound)
		},
	} {
		if _, err := serve(t, handler).Fetch(context.Background(), []string{"openrouter"}, ""); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
}
