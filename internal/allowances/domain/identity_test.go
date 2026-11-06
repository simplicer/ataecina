package domain_test

import (
	"errors"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func TestSubscriptionIDAcceptsOwnerChosenNames(t *testing.T) {
	for _, raw := range []string{"claude", "codex-work", "zai", "z1"} {
		id, err := domain.NewSubscriptionID(raw)
		if err != nil {
			t.Fatalf("NewSubscriptionID(%q) = %v", raw, err)
		}
		if id.String() != raw {
			t.Fatalf("String() = %q, want %q", id, raw)
		}
	}
}

func TestSubscriptionIDRejectsUnsafeNames(t *testing.T) {
	for _, raw := range []string{"", "Claude", "../keys", "-claude", "a b", "abcdefghijklmnopqrstuvwxyz0123456789"} {
		if _, err := domain.NewSubscriptionID(raw); !errors.Is(err, domain.ErrInvalidIdentifier) {
			t.Fatalf("NewSubscriptionID(%q) = %v, want ErrInvalidIdentifier", raw, err)
		}
	}
}

func TestProviderAndSourceIdentifiers(t *testing.T) {
	provider, err := domain.NewProviderID("anthropic")
	if err != nil || provider.String() != "anthropic" {
		t.Fatalf("NewProviderID = %q, %v", provider, err)
	}
	if _, err := domain.NewProviderID("Anthropic"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatalf("NewProviderID(Anthropic) = %v", err)
	}
	source, err := domain.NewSourceID("claude-statusline")
	if err != nil {
		t.Fatalf("NewSourceID = %v", err)
	}
	if source.Label() != "CLAUDE_STATUSLINE" || source.String() != "claude-statusline" {
		t.Fatalf("Label() = %q, String() = %q", source.Label(), source)
	}
	if _, err := domain.NewSourceID("claude_statusline"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatalf("NewSourceID(claude_statusline) = %v", err)
	}
}

func TestPlanIsNormalisedToUpperSnakeCase(t *testing.T) {
	cases := map[string]string{
		"plus":             "PLUS",
		" Pro ":            "PRO",
		"max 20x":          "MAX_20X",
		"coding-plan.lite": "CODING_PLAN_LITE",
		"":                 "",
		"--":               "",
	}
	for raw, want := range cases {
		if got := domain.NewPlan(raw); got.String() != want {
			t.Fatalf("NewPlan(%q) = %q, want %q", raw, got, want)
		}
	}
	if !domain.NewPlan("").IsUnknown() || domain.NewPlan("pro").IsUnknown() {
		t.Fatal("IsUnknown is wrong")
	}
	long := domain.NewPlan("a-very-long-plan-name-that-goes-on-and-on-forever")
	if len(long.String()) > 32 {
		t.Fatalf("plan not capped: %q", long)
	}
}

func TestBucketIsNormalised(t *testing.T) {
	if got := domain.NewBucketID(" Premium "); got.String() != "premium" {
		t.Fatalf("NewBucketID = %q", got)
	}
	if got := domain.NewBucketID("base_model_inference"); got.String() != "base_model_inference" {
		t.Fatalf("NewBucketID = %q", got)
	}
	if got := domain.NewBucketID("a/b c"); got.String() != "a_b_c" {
		t.Fatalf("NewBucketID = %q", got)
	}
	if !domain.NewBucketID("").IsMain() {
		t.Fatal("empty bucket must be the main bucket")
	}
}
