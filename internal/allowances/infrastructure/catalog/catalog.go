// Package catalog lists every source Ataecina knows (SPECS §3): its
// provider, the fidelity it gives, whether it needs a sealed key and which
// extra options its subscriptions accept in config.json.
package catalog

import "codeberg.org/simplicer/ataecina/internal/allowances/domain"

// Option names a source-specific setting in config.json.
type Option string

// Source-specific options.
const (
	OptionAppServer        Option = "app_server"
	OptionAppServerAfter   Option = "app_server_after_minutes"
	OptionProvider         Option = "provider"
	OptionAccount          Option = "account"
	OptionLogin            Option = "login"
	OptionMonthlyAllowance Option = "monthly_allowance"
	OptionPort             Option = "port"
	OptionTeam             Option = "team"
	OptionEmail            Option = "email"
)

// Entry describes one source.
type Entry struct {
	Source   domain.SourceID
	Provider domain.ProviderID // empty when the owner must declare it
	Fidelity domain.Fidelity
	NeedsKey bool
	Options  []Option
	// Requires lists the options a subscription of this source must set.
	Requires []Option
	// TakesDeclarations marks balance-only sources that derive what is
	// left from `ataecina balance declare`.
	TakesDeclarations bool
}

// Accepts reports whether the source takes an option.
func (e Entry) Accepts(option Option) bool {
	for _, accepted := range e.Options {
		if accepted == option {
			return true
		}
	}
	return false
}

// Catalog is a set of sources.
type Catalog struct {
	entries map[domain.SourceID]Entry
}

// Standard is the catalog of the production build.
func Standard() Catalog {
	reported, derived, observed := domain.FidelityReported, domain.FidelityDerived, domain.FidelityObserved
	entries := []Entry{
		{Source: "claude-statusline", Provider: "anthropic", Fidelity: reported},
		{Source: "codex-logs", Provider: "openai", Fidelity: reported, Options: []Option{OptionAppServer, OptionAppServerAfter}},
		{Source: "codex-app-server", Provider: "openai", Fidelity: reported},
		{Source: "zai-api", Provider: "zai", Fidelity: reported, NeedsKey: true},
		{Source: "minimax-api", Provider: "minimax", Fidelity: reported, NeedsKey: true},
		{Source: "alibaba-cli", Provider: "alibaba", Fidelity: reported},
		{Source: "commandcode-api", Provider: "commandcode", Fidelity: reported, NeedsKey: true},
		{Source: "openrouter-api", Provider: "openrouter", Fidelity: reported, NeedsKey: true},
		{Source: "vercel-api", Provider: "vercel", Fidelity: reported, NeedsKey: true},
		{Source: "opencode-zen", Provider: "opencode", Fidelity: derived, TakesDeclarations: true},
		{Source: "opencode-console", Provider: "opencode", Fidelity: reported},
		{Source: "ollama-api", Provider: "ollama", Fidelity: reported, NeedsKey: true},
		{Source: "copilot-gh", Provider: "github", Fidelity: derived,
			Options: []Option{OptionLogin, OptionMonthlyAllowance}, Requires: []Option{OptionLogin, OptionMonthlyAllowance}},
		{Source: "orcarouter-api", Provider: "orcarouter", Fidelity: reported, NeedsKey: true},
		{Source: "moonshot-api", Provider: "moonshot", Fidelity: reported, NeedsKey: true},
		{Source: "moonshot-cn-api", Provider: "moonshot", Fidelity: reported, NeedsKey: true},
		{Source: "kimi-code-api", Provider: "kimi", Fidelity: reported, NeedsKey: true},
		{Source: "xai-api", Provider: "xai", Fidelity: reported, NeedsKey: true,
			Options: []Option{OptionTeam}, Requires: []Option{OptionTeam}},
		{Source: "cursor-admin-api", Provider: "cursor", Fidelity: reported, NeedsKey: true,
			Options: []Option{OptionEmail}, Requires: []Option{OptionEmail}},
		{Source: "cloudflare-api", Provider: "cloudflare", Fidelity: reported, NeedsKey: true,
			Options: []Option{OptionAccount}, Requires: []Option{OptionAccount}},
		{Source: "heimdall", Fidelity: observed, NeedsKey: true, Options: []Option{OptionProvider, OptionPort}},
	}
	catalog := Catalog{entries: make(map[domain.SourceID]Entry, len(entries))}
	for _, entry := range entries {
		catalog.entries[entry.Source] = entry
	}
	return catalog
}

// With returns a copy of the catalog with one more source.
func (c Catalog) With(entry Entry) Catalog {
	entries := make(map[domain.SourceID]Entry, len(c.entries)+1)
	for id, existing := range c.entries {
		entries[id] = existing
	}
	entries[entry.Source] = entry
	return Catalog{entries: entries}
}

// Lookup finds a source.
func (c Catalog) Lookup(source domain.SourceID) (Entry, bool) {
	entry, ok := c.entries[source]
	return entry, ok
}
