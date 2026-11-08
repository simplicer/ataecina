package domain_test

import (
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

func TestAModelIsRecognisedAcrossCatalogues(t *testing.T) {
	for text, want := range map[string]string{
		"z-ai/glm-5.3:free":        "glm-5-3",
		"glm-5.3":                  "glm-5-3",
		"GLM-5.3 (Max)":            "glm-5-3",
		"deepseek-v4-flash-free":   "deepseek-v4-flash",
		"Kimi K3.1 Pro":            "kimi-k3-1-pro",
		"  ":                       "",
		"openai/gpt-6.1-terra":     "gpt-6-1-terra",
		"MiniMax-M3.1-Flash Free":  "minimax-m3-1-flash",
		"z-ai/glm-5.3-20260816":    "glm-5-3",
		"deepseek-v4.1-flash-1017": "deepseek-v4-1-flash-1017",
	} {
		if got := domain.ModelKey(text); got != want {
			t.Fatalf("ModelKey(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestTheBestVariantIsTheIndex(t *testing.T) {
	indexer := domain.Ranking{Scores: []domain.Score{
		{Name: "GLM-5.3 (Low)", Index: 52}, {Name: "GLM-5.3 (Max)", Index: 61}, {Name: "", Index: 99},
		{Name: "Kimi K3", Index: 58},
	}}.Indexer()
	if index, ok := indexer.IndexOf(domain.Offering{Model: "z-ai/glm-5.3"}); !ok || index != 61 {
		t.Fatalf("by id = %v, %v", index, ok)
	}
	if index, ok := indexer.IndexOf(domain.Offering{Model: "k3", Name: "Kimi K3"}); !ok || index != 58 {
		t.Fatalf("by name = %v, %v", index, ok)
	}
	if _, ok := indexer.IndexOf(domain.Offering{Model: "unknown", Name: ""}); ok {
		t.Fatal("an unknown model was ranked")
	}
}
