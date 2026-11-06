package configfile_test

import (
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
)

// FuzzDecode checks that no configuration, however malformed, panics the
// decoder or yields a subscription without an id, provider or source.
func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		`{"version": 1}`,
		`{"version": 1, "subscriptions": [{"id": "claude", "source": "claude-statusline", "plan": "PRO"}]}`,
		`{"version": 1, "interval_minutes": 5, "alerts": {"thresholds": [90, 95]}, "subscriptions": [{"id": "codex", "source": "codex-logs", "app_server": true}]}`,
		`{"version": 1, "subscriptions": [{"id": "g", "source": "heimdall", "provider": "google"}]}`,
		`{"version": 2}`,
		`[]`,
		``,
	} {
		f.Add([]byte(seed))
	}
	sources := catalog.Standard()
	f.Fuzz(func(t *testing.T, data []byte) {
		settings, err := configfile.Decode(data, sources)
		if err != nil {
			return
		}
		for _, item := range settings.Subscriptions {
			s := item.Subscription
			if s.ID() == "" || s.Provider() == "" || s.Source() == "" {
				t.Fatalf("incomplete subscription from %q", data)
			}
		}
	})
}
