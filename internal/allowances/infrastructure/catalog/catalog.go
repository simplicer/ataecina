// Package catalog lists every source Ataecina knows (SPECS §3): its
// provider, the fidelity it gives, whether it needs a sealed key and which
// extra options its subscriptions accept in config.json.
package catalog

import "codeberg.org/simplicer/ataecina/internal/allowances/domain"

// Option names a source-specific setting in config.json.
type Option string

// Source-specific options.
const (
	OptionAppServer      Option = "app_server"
	OptionAppServerAfter Option = "app_server_after_minutes"
	OptionProvider       Option = "provider"
)

// Entry describes one source.
type Entry struct {
	Source   domain.SourceID
	Provider domain.ProviderID // empty when the owner must declare it
	Fidelity domain.Fidelity
	NeedsKey bool
	Options  []Option
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
		{Source: "opencode-zen", Provider: "opencode", Fidelity: derived},
		{Source: "ollama-api", Provider: "ollama", Fidelity: reported, NeedsKey: true},
		{Source: "copilot-gh", Provider: "github", Fidelity: derived},
		{Source: "orcarouter-api", Provider: "orcarouter", Fidelity: reported, NeedsKey: true},
		{Source: "cloudflare-api", Provider: "cloudflare", Fidelity: reported, NeedsKey: true},
		{Source: "heimdall", Fidelity: observed, NeedsKey: true, Options: []Option{OptionProvider}},
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
