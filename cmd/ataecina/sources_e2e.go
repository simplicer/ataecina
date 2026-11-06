//go:build e2e

package main

import (
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/fakesource"
)

// buildSources adds the fake provider that only e2e binaries contain
// (AGENTS.md: production builds cannot redirect a source).
func buildSources() sourceRegistry {
	return standardSources().with(
		catalog.Entry{Source: "e2e-fake", Provider: "e2e", Fidelity: domain.FidelityReported},
		func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			return fakesource.New(c.env.getenv("ATAECINA_E2E_FAKES"))
		},
	)
}

// plainHTTPAllowed lets e2e binaries reach a fake provider on http://127.0.0.1.
const plainHTTPAllowed = true

// providerOrigin sends every API adapter of an e2e binary to the fake
// provider in ATAECINA_E2E_PROVIDER_ORIGIN, when set.
func providerOrigin(env environment, compiled string) string {
	if fake := env.getenv("ATAECINA_E2E_PROVIDER_ORIGIN"); fake != "" {
		return fake
	}
	return compiled
}
