package configfile_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
)

func decode(t *testing.T, text string) (configfile.Settings, error) {
	t.Helper()
	return configfile.Decode([]byte(text), catalog.Standard())
}

func TestAMinimalConfigurationGetsTheDefaults(t *testing.T) {
	settings, err := decode(t, `{"version": 1, "subscriptions": [{"id": "claude", "source": "claude-statusline"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Interval != 5*time.Minute || !reflect.DeepEqual(settings.Thresholds, []int{90, 95}) || settings.UseEnvironmentProxy {
		t.Fatalf("defaults = %+v", settings)
	}
	claude := settings.Subscriptions[0]
	if claude.Subscription.Provider() != "anthropic" || claude.Subscription.Fidelity() != domain.FidelityReported || claude.Key != "" {
		t.Fatalf("claude = %+v", claude)
	}
	if got := settings.DomainSubscriptions(); len(got) != 1 || got[0].ID() != "claude" {
		t.Fatalf("DomainSubscriptions() = %v", got)
	}
}

func TestEveryOptionIsRead(t *testing.T) {
	settings, err := decode(t, `{
	  "version": 1, "interval_minutes": 10,
	  "alerts": {"thresholds": [80, 90, 99]},
	  "network": {"use_environment_proxy": true},
	  "subscriptions": [
	    {"id": "codex", "source": "codex-logs", "plan": "plus", "app_server": true, "app_server_after_minutes": 30},
	    {"id": "zai", "source": "zai-api", "key": "zai-coding"},
	    {"id": "openrouter", "source": "openrouter-api"},
	    {"id": "gemini", "source": "heimdall", "provider": "google"}
	  ]}`)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Interval != 10*time.Minute || !settings.UseEnvironmentProxy || len(settings.Thresholds) != 3 {
		t.Fatalf("settings = %+v", settings)
	}
	codex, zai, openrouter, gemini := settings.Subscriptions[0], settings.Subscriptions[1], settings.Subscriptions[2], settings.Subscriptions[3]
	if !codex.AppServer || codex.AppServerAfter != 30*time.Minute || codex.Subscription.Plan() != "PLUS" {
		t.Fatalf("codex = %+v", codex)
	}
	if zai.Key != "zai-coding" || openrouter.Key != "openrouter" {
		t.Fatalf("keys = %q, %q", zai.Key, openrouter.Key)
	}
	if gemini.Subscription.Provider() != "google" || gemini.Subscription.Fidelity() != domain.FidelityObserved {
		t.Fatalf("gemini = %+v", gemini)
	}
}

func TestStrictValidation(t *testing.T) {
	cases := map[string]string{
		"not json":            `{`,
		"trailing data":       `{"version": 1} {}`,
		"unknown key":         `{"version": 1, "colour": "red"}`,
		"unknown sub key":     `{"version": 1, "subscriptions": [{"id": "a", "source": "zai-api", "token": "x"}]}`,
		"no version":          `{"subscriptions": []}`,
		"wrong version":       `{"version": 2}`,
		"interval zero":       `{"version": 1, "interval_minutes": 0}`,
		"interval too long":   `{"version": 1, "interval_minutes": 2000}`,
		"no thresholds":       `{"version": 1, "alerts": {"thresholds": []}}`,
		"falling thresholds":  `{"version": 1, "alerts": {"thresholds": [95, 90]}}`,
		"threshold too high":  `{"version": 1, "alerts": {"thresholds": [101]}}`,
		"bad id":              `{"version": 1, "subscriptions": [{"id": "Claude", "source": "claude-statusline"}]}`,
		"bad source":          `{"version": 1, "subscriptions": [{"id": "a", "source": "Claude"}]}`,
		"unknown source":      `{"version": 1, "subscriptions": [{"id": "a", "source": "chatgpt-web"}]}`,
		"duplicate id":        `{"version": 1, "subscriptions": [{"id": "a", "source": "zai-api"}, {"id": "a", "source": "zai-api"}]}`,
		"foreign option":      `{"version": 1, "subscriptions": [{"id": "a", "source": "zai-api", "app_server": true}]}`,
		"key on local":        `{"version": 1, "subscriptions": [{"id": "a", "source": "claude-statusline", "key": "a"}]}`,
		"bad key name":        `{"version": 1, "subscriptions": [{"id": "a", "source": "zai-api", "key": "../x"}]}`,
		"no provider":         `{"version": 1, "subscriptions": [{"id": "a", "source": "heimdall"}]}`,
		"bad provider":        `{"version": 1, "subscriptions": [{"id": "a", "source": "heimdall", "provider": "Google"}]}`,
		"provider elsewhere":  `{"version": 1, "subscriptions": [{"id": "a", "source": "zai-api", "provider": "x"}]}`,
		"app server too soon": `{"version": 1, "subscriptions": [{"id": "a", "source": "codex-logs", "app_server_after_minutes": 0}]}`,
	}
	for name, text := range cases {
		if _, err := decode(t, text); !errors.Is(err, configfile.ErrInvalid) {
			t.Fatalf("%s: Decode = %v, want ErrInvalid", name, err)
		}
	}
}

func TestErrorsNameTheProblem(t *testing.T) {
	_, err := decode(t, `{"version": 1, "subscriptions": [{"id": "a", "source": "chatgpt-web"}]}`)
	if err == nil || !strings.Contains(err.Error(), `unknown source "chatgpt-web"`) {
		t.Fatalf("error = %v", err)
	}
}
