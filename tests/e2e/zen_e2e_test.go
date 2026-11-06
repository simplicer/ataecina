//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installFakeOpencode(t *testing.T, tr *tree, stats string) string {
	t.Helper()
	script, err := os.ReadFile("testdata/fake-opencode")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tr.root, "bin")
	mustPrivateDir(t, bin)
	if err := os.WriteFile(filepath.Join(bin, "opencode"), script, 0o700); err != nil {
		t.Fatal(err)
	}
	statsFile := filepath.Join(tr.root, "opencode-stats.json")
	if err := os.WriteFile(statsFile, []byte(stats), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(tr.root, "opencode-argv")
	tr.setenv("PATH", bin+":/usr/bin:/bin")
	tr.setenv("FAKE_OPENCODE_STATS", statsFile)
	tr.setenv("FAKE_OPENCODE_JOURNAL", journal)
	return journal
}

func TestOpenCodeZenEndToEnd(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "zen", "source": "opencode-zen"}]}`, nil)
	journal := installFakeOpencode(t, tr, `{"models": [{"model": {"providerID": "opencode", "modelID": "zen-synthetic"}, "cost": 1.5}]}`)

	code, out, stderr := tr.run(bin, "balance", "declare", "zen", "--top-up", "10")
	if code != 0 || !strings.Contains(out, "declared top-up 10 USD") {
		t.Fatalf("declare = %d %q %q", code, out, stderr)
	}
	if err := os.WriteFile(filepath.Join(tr.root, "opencode-stats.json"), []byte(`{"models": [{"model": {"providerID": "opencode", "modelID": "zen-synthetic"}, "cost": 4}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	_, status, _ := tr.run(bin, "status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []struct {
				Fidelity string `json:"fidelity"`
				Source   string `json:"source"`
				Balance  struct {
					Remaining struct {
						Amount   float64 `json:"amount"`
						Currency string  `json:"currency"`
					} `json:"remaining"`
				} `json:"balance"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status), &doc); err != nil {
		t.Fatal(err)
	}
	zen := doc.Data.Subscriptions[0]
	if zen.Fidelity != "DERIVED" || zen.Source != "OPENCODE_ZEN" || zen.Balance.Remaining.Amount != 7.5 || zen.Balance.Remaining.Currency != "USD" {
		t.Fatalf("zen = %+v", zen)
	}
	argv, _ := os.ReadFile(journal)
	for _, line := range strings.Split(strings.TrimSpace(string(argv)), "\n") {
		if line != "stats --all --models --limit 1000 --json --standalone" {
			t.Fatalf("opencode was run with %q", line)
		}
	}
	info, err := os.Stat(filepath.Join(tr.root, "state", "ataecina", "balances.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("balances.json = %v", err)
	}
}
