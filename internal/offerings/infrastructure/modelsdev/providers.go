package modelsdev

// bySource is the models.dev provider that lists the models of each
// source's subscriptions (provider ids checked against the catalogue on
// 2026-11-08). Sources missing here (Claude, Codex) have no catalogue
// provider that describes what their plans include.
var bySource = map[string]string{
	"zai-api":          "zai-coding-plan",
	"minimax-api":      "minimax-coding-plan",
	"alibaba-cli":      "alibaba-token-plan",
	"commandcode-api":  "commandcode",
	"openrouter-api":   "openrouter",
	"vercel-api":       "vercel",
	"opencode-zen":     "opencode",
	"opencode-console": "opencode-go",
	"ollama-api":       "ollama-cloud",
	"copilot-gh":       "github-copilot",
	"orcarouter-api":   "orcarouter",
	"cloudflare-api":   "cloudflare-workers-ai",
	"moonshot-api":     "moonshotai",
	"moonshot-cn-api":  "moonshotai-cn",
	"kimi-code-api":    "kimi-code-plan-global",
}

// ProviderFor names the catalogue provider whose models describe a
// subscription: the one the owner declared in config.json, else the one
// of its source; for Heimdall, the provider it follows (google, nvidia,
// groq…). Empty when there is none.
func ProviderFor(source, provider, declared string) string {
	switch {
	case declared != "":
		return declared
	case source == "heimdall" && providerID.MatchString(provider):
		return provider
	}
	return bySource[source]
}

// ValidProvider tells whether a declared catalogue provider id is plain.
func ValidProvider(id string) bool { return providerID.MatchString(id) }
