//go:build e2e

package main

import (
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/fakesource"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// buildSources adds the fake provider that only e2e binaries contain
// (AGENTS.md: production builds cannot redirect a source).
func buildSources() sourceRegistry {
	return standardSources().with(
		catalog.Entry{Source: "e2e-fake", Provider: "e2e", Fidelity: domain.FidelityReported},
		func(_ configfile.SubscriptionSettings, _ xdg.Paths, env environment) domain.Source {
			return fakesource.New(env.getenv("ATAECINA_E2E_FAKES"))
		},
	)
}
