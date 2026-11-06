//go:build acceptance

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeOpencode installs tests/e2e/testdata/fake-opencode as `opencode` on
// the world's PATH, reporting spent dollars on OpenCode Zen to date.
func (w *world) fakeOpencode(spent string) {
	w.t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "tests", "e2e", "testdata", "fake-opencode"))
	if err != nil {
		w.t.Fatal(err)
	}
	bin := filepath.Join(w.root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "opencode"), script, 0o700); err != nil {
		w.t.Fatal(err)
	}
	stats := filepath.Join(w.root, "opencode-stats.json")
	output := fmt.Sprintf(`{"models": [{"model": {"providerID": "opencode", "modelID": "zen-synthetic"}, "cost": %s}, {"model": {"providerID": "openrouter", "modelID": "synthetic"}, "cost": 9}]}`, spent)
	if err := os.WriteFile(stats, []byte(output), 0o600); err != nil {
		w.t.Fatal(err)
	}
	w.t.Setenv("FAKE_OPENCODE_STATS", stats) // read by the script itself
	w.vars["PATH"] = bin + ":/usr/bin:/bin"
}

const zenConfig = `{"version": 1, "subscriptions": [
  {"id": "zen", "source": "opencode-zen"},
  {"id": "codex", "source": "codex-logs"}
]}`

type zenStatus struct {
	Outcome  string  `json:"outcome"`
	Fidelity string  `json:"fidelity"`
	Hint     *string `json:"hint"`
	Balance  *struct {
		Remaining *struct {
			Amount float64 `json:"amount"`
		} `json:"remaining"`
		Spent *struct {
			Amount float64 `json:"amount"`
		} `json:"spent"`
	} `json:"balance"`
}

func (w *world) zen() zenStatus {
	w.t.Helper()
	_, out, _ := w.run("status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []zenStatus `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		w.t.Fatal(err)
	}
	return doc.Data.Subscriptions[0]
}

func TestTheZenBalanceIsDerivedFromDeclaredTopUps(t *testing.T) {
	w := newWorld(t, zenConfig)
	w.fakeOpencode("4.75")
	w.collect()
	if zen := w.zen(); zen.Outcome != "PARTIAL" || zen.Balance.Remaining != nil || zen.Balance.Spent.Amount != 4.75 || zen.Hint == nil {
		t.Fatalf("before any declaration: %+v", zen)
	}

	code, out, stderr := w.run("balance", "declare", "zen", "--top-up", "20")
	if code != exitOK || strings.TrimSpace(out) != "zen: declared top-up 20 USD (spent to date: 4.75 USD)" {
		t.Fatalf("declare = %d %q %q", code, out, stderr)
	}
	w.fakeOpencode("6.75")
	w.collect()
	zen := w.zen()
	if zen.Outcome != "OK" || zen.Fidelity != "DERIVED" || zen.Balance.Remaining.Amount != 18 || zen.Balance.Spent.Amount != 2 {
		t.Fatalf("after a top-up: %+v", zen)
	}
	_, table, _ := w.run()
	if !strings.Contains(table, "≈ $18.00 left · $2.00 spent") {
		t.Fatalf("table:\n%s", table)
	}

	if code, _, _ := w.run("balance", "declare", "zen", "--balance", "5"); code != exitOK {
		t.Fatal("a declared balance replaces the top-ups")
	}
	w.collect()
	if zen := w.zen(); zen.Balance.Remaining.Amount != 5 || zen.Balance.Spent.Amount != 0 {
		t.Fatalf("after a declared balance: %+v", zen)
	}
}

func TestBalanceDeclarationsAreChecked(t *testing.T) {
	w := newWorld(t, zenConfig)
	w.fakeOpencode("1")
	cases := []struct {
		args []string
		code int
	}{
		{[]string{"balance"}, exitUsage},
		{[]string{"balance", "declare"}, exitUsage},
		{[]string{"balance", "declare", "--top-up", "5"}, exitUsage},
		{[]string{"balance", "declare", "zen"}, exitUsage},
		{[]string{"balance", "declare", "zen", "--top-up", "5", "--balance", "5"}, exitUsage},
		{[]string{"balance", "declare", "zen", "--top-up", "5", "extra"}, exitUsage},
		{[]string{"balance", "declare", "zen", "--top-up", "five"}, exitFailure},
		{[]string{"balance", "declare", "zen", "--top-up", "0"}, exitFailure},
		{[]string{"balance", "declare", "zen", "--top-up", "5", "--currency", "usd"}, exitFailure},
		{[]string{"balance", "declare", "zen", "--top-up", "5", "--currency", "EUR"}, exitFailure},
		{[]string{"balance", "declare", "Zen", "--top-up", "5"}, exitFailure},
		{[]string{"balance", "declare", "absent", "--top-up", "5"}, exitFailure},
		{[]string{"balance", "declare", "codex", "--top-up", "5"}, exitFailure},
	}
	for _, c := range cases {
		if code, _, _ := w.run(c.args...); code != c.code {
			t.Fatalf("%v = %d, want %d", c.args, code, c.code)
		}
	}
	w.vars["PATH"] = "/usr/bin:/bin"
	if code, _, stderr := w.run("balance", "declare", "zen", "--top-up", "5"); code != exitFailure || !strings.Contains(stderr, "opencode is not on PATH") {
		t.Fatalf("without opencode = %d %q", code, stderr)
	}
	delete(w.vars, "HOME")
	if code, _, _ := w.run("balance", "declare", "zen", "--top-up", "5"); code != exitFailure {
		t.Fatal("without HOME the declaration fails")
	}
}
