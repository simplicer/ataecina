//go:build integration

package configfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
)

const original = `{
  "version": 1,
  "interval_minutes": 5,
  "alerts": { "thresholds": [90, 95] },
  "subscriptions": [
    { "id": "minimax", "source": "minimax-api" },
    { "id": "zen", "source": "opencode-zen" }
  ]
}
`

func configIn(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ataecina")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestASecondSubscriptionOfTheSameSourceIsAdded(t *testing.T) {
	path := configIn(t)
	err := configfile.AddSubscription(path, catalog.Standard(), configfile.NewSubscription{
		ID: "minimax-work", Source: "minimax-api", Key: "minimax-work", Plan: "Plus",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := configfile.AddSubscription(path, catalog.Standard(), configfile.NewSubscription{
		ID: "copilot", Source: "copilot-gh", Options: map[string]string{"login": "octo-cat", "monthly_allowance": "300"},
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := configfile.Load(path, catalog.Standard())
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Subscriptions) != 4 || settings.Subscriptions[2].Key != "minimax-work" || settings.Subscriptions[3].MonthlyAllowance != 300 {
		t.Fatalf("subscriptions = %+v", settings.Subscriptions)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `    { "id": "minimax-work", "source": "minimax-api", "plan": "Plus", "key": "minimax-work" },`) ||
		!strings.Contains(string(data), `  "alerts": { "thresholds": [90, 95] },`) {
		t.Fatalf("config.json =\n%s", data)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode())
	}
}

func TestAnInvalidChangeLeavesTheFileAlone(t *testing.T) {
	path := configIn(t)
	for name, item := range map[string]configfile.NewSubscription{
		"duplicate id":    {ID: "zen", Source: "opencode-zen"},
		"unknown source":  {ID: "web", Source: "chatgpt-web"},
		"missing option":  {ID: "xai", Source: "xai-api"},
		"foreign option":  {ID: "z", Source: "zai-api", Options: map[string]string{"team": "t"}},
		"number as text":  {ID: "c", Source: "copilot-gh", Options: map[string]string{"login": "o", "monthly_allowance": "many"}},
		"switch as text":  {ID: "codex", Source: "codex-logs", Options: map[string]string{"app_server": "sometimes"}},
		"id with a slash": {ID: "a/b", Source: "zai-api"},
		"key on no-key":   {ID: "z2", Source: "opencode-zen", Key: "z2"},
	} {
		if err := configfile.AddSubscription(path, catalog.Standard(), item); !errors.Is(err, configfile.ErrInvalid) {
			t.Fatalf("%s: AddSubscription = %v", name, err)
		}
	}
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatalf("the file changed:\n%s", data)
	}
}

func TestASubscriptionIsRemoved(t *testing.T) {
	path := configIn(t)
	if err := configfile.RemoveSubscription(path, catalog.Standard(), "zen"); err != nil {
		t.Fatal(err)
	}
	settings, err := configfile.Load(path, catalog.Standard())
	if err != nil || len(settings.Subscriptions) != 1 || settings.Subscriptions[0].Subscription.ID() != "minimax" {
		t.Fatalf("after removing = %+v, %v", settings.Subscriptions, err)
	}
	if err := configfile.RemoveSubscription(path, catalog.Standard(), "zen"); !errors.Is(err, configfile.ErrNoSuchSubscription) {
		t.Fatalf("removing twice = %v", err)
	}
}
