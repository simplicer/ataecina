//go:build !e2e

package main

import "testing"

// TestProductionOriginsCannotBeRedirected guards ADR 0006: whatever the
// environment says, a production binary uses the compiled-in origin and
// speaks HTTPS only.
func TestProductionOriginsCannotBeRedirected(t *testing.T) {
	env := environment{getenv: func(string) string { return "http://127.0.0.1:9" }}
	if got := providerOrigin(env, "https://openrouter.ai"); got != "https://openrouter.ai" {
		t.Fatalf("providerOrigin = %q", got)
	}
	if plainHTTPAllowed {
		t.Fatal("production must refuse plain HTTP")
	}
}
