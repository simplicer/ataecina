package modelsdev_test

import (
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/modelsdev"
)

func TestEachSubscriptionFindsItsCatalogueProvider(t *testing.T) {
	for _, c := range []struct{ source, provider, declared, want string }{
		{"zai-api", "zai", "", "zai-coding-plan"},
		{"opencode-console", "opencode", "", "opencode-go"},
		{"alibaba-cli", "alibaba", "alibaba-coding-plan", "alibaba-coding-plan"},
		{"heimdall", "nvidia", "", "nvidia"},
		{"heimdall", "Not An Id", "", ""},
		{"claude-statusline", "anthropic", "", ""},
	} {
		if got := modelsdev.ProviderFor(c.source, c.provider, c.declared); got != c.want {
			t.Fatalf("ProviderFor(%s, %s, %q) = %q, want %q", c.source, c.provider, c.declared, got, c.want)
		}
	}
	if !modelsdev.ValidProvider("kimi-code-plan-global") || modelsdev.ValidProvider("../x") || modelsdev.ValidProvider("") {
		t.Fatal("ValidProvider misjudged")
	}
}
