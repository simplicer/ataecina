//go:build integration

package httpclient_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

func endpoint(t *testing.T, handler http.HandlerFunc, options httpclient.Options) httpclient.Endpoint {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	options.AllowPlainHTTP = true
	e, err := httpclient.New(options).For(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestRequestsCarryOnlyAtaecinasIdentity(t *testing.T) {
	var got http.Header
	var path string
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		got, path = r.Header.Clone(), r.URL.RequestURI()
		w.Write([]byte(`{"ok":true}`))
	}, httpclient.Options{Version: "0.9.0-unstable"})
	response, err := e.Get(context.Background(), "/v1/credits?x=1", http.Header{"Authorization": {"Bearer k"}})
	if err != nil || response.Status != 200 || string(response.Body) != `{"ok":true}` {
		t.Fatalf("Get = %+v, %v", response, err)
	}
	if got.Get("User-Agent") != "ataecina/0.9.0-unstable" || got.Get("Accept") != "application/json" || got.Get("Authorization") != "Bearer k" || path != "/v1/credits?x=1" {
		t.Fatalf("request = %v %s", got, path)
	}
}

func TestRedirectsAreRefused(t *testing.T) {
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example/steal", http.StatusFound)
	}, httpclient.Options{})
	if _, err := e.Get(context.Background(), "/", nil); !errors.Is(err, httpclient.ErrRedirected) {
		t.Fatalf("redirect = %v", err)
	}
}

func TestBodiesAreCappedAfterDecompression(t *testing.T) {
	var zipped bytes.Buffer
	writer := gzip.NewWriter(&zipped)
	writer.Write(bytes.Repeat([]byte("x"), httpclient.MaxBodyBytes+1))
	writer.Close()
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(zipped.Bytes())
	}, httpclient.Options{})
	if zipped.Len() >= httpclient.MaxBodyBytes {
		t.Fatal("the compressed body must be small to test the cap after decompression")
	}
	if _, err := e.Get(context.Background(), "/", nil); !errors.Is(err, httpclient.ErrTooLarge) {
		t.Fatalf("large body = %v", err)
	}
	larger := e.Larger(2*httpclient.MaxBodyBytes, time.Second)
	if response, err := larger.Get(context.Background(), "/", nil); err != nil || len(response.Body) != httpclient.MaxBodyBytes+1 {
		t.Fatalf("larger endpoint = %d bytes, %v", len(response.Body), err)
	}
	if _, err := e.Larger(1, time.Millisecond).Get(context.Background(), "/", nil); !errors.Is(err, httpclient.ErrTooLarge) {
		t.Fatalf("Larger lowered the cap: %v", err)
	}
}

func TestEachEndpointHasItsOwnDeadline(t *testing.T) {
	release := make(chan struct{})
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}, httpclient.Options{})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := e.Get(ctx, "/", nil); err == nil {
		t.Fatal("a request outlived its deadline")
	}
}

func TestProxiesAreIgnoredUnlessEnabled(t *testing.T) {
	proxied := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxied = true }))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("direct")) }, httpclient.Options{})
	if response, err := e.Get(context.Background(), "/", nil); err != nil || string(response.Body) != "direct" || proxied {
		t.Fatalf("proxy used without opt-in: %v %v", err, proxied)
	}
}

func TestSlowProvidersTimeOutAndCancelStops(t *testing.T) {
	release := make(chan struct{})
	e := endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}, httpclient.Options{})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := e.Get(ctx, "/", nil); err == nil {
		t.Fatal("a cancelled request must fail")
	}
}

func TestOriginsArePinned(t *testing.T) {
	production := httpclient.New(httpclient.Options{})
	for _, origin := range []string{"http://openrouter.ai", "https://user:pw@openrouter.ai", "https://openrouter.ai/api", "https://openrouter.ai?x=1", "openrouter.ai", ":"} {
		if _, err := production.For(origin); !errors.Is(err, httpclient.ErrBadOrigin) {
			t.Fatalf("For(%q) = %v", origin, err)
		}
	}
	e, err := production.For("https://openrouter.ai/")
	if err != nil || e.Host() != "openrouter.ai" {
		t.Fatalf("For = %v", err)
	}
	for _, path := range []string{"api/v1/key", "//evil.example/x", "https://evil.example/x"} {
		if _, err := e.Get(context.Background(), path, nil); !errors.Is(err, httpclient.ErrBadOrigin) {
			t.Fatalf("Get(%q) = %v", path, err)
		}
	}
	if _, err := e.Do(context.Background(), "BAD METHOD", "/x", nil, strings.NewReader("")); err == nil {
		t.Fatal("an invalid method must fail")
	}
}

func TestOnlyLoopbackMaySpeakPlainHTTPInProduction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)
	production := httpclient.New(httpclient.Options{Version: "test"})
	if _, err := production.For(server.URL); !errors.Is(err, httpclient.ErrBadOrigin) {
		t.Fatalf("plain HTTP elsewhere = %v", err)
	}
	port := server.Listener.Addr().(*net.TCPAddr).Port
	e, err := production.ForLoopback(port)
	if err != nil || e.Host() != "127.0.0.1:"+strconv.Itoa(port) {
		t.Fatalf("ForLoopback = %v, %v", e.Host(), err)
	}
	if response, err := e.Get(context.Background(), "/v1/plans", nil); err != nil || response.Status != 200 {
		t.Fatalf("Get = %+v, %v", response, err)
	}
	for _, port := range []int{0, 70000} {
		if _, err := production.ForLoopback(port); !errors.Is(err, httpclient.ErrBadOrigin) {
			t.Fatalf("port %d = %v", port, err)
		}
	}
}
