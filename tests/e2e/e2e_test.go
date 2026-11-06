//go:build e2e

// Package e2e runs the built binaries end to end in a throw-away HOME and
// XDG tree against fake providers. `make e2e` builds the e2e binary (which
// contains the fake source) and the production binary (which must not).
package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type tree struct {
	t    *testing.T
	root string
	env  []string
}

func binary(t *testing.T, variable string) string {
	t.Helper()
	path := os.Getenv(variable)
	if path == "" {
		t.Skipf("%s is not set; run make e2e", variable)
	}
	return path
}

func newTree(t *testing.T, config string, fakes map[string]string) *tree {
	t.Helper()
	root := t.TempDir()
	dirs := map[string]string{
		"HOME":            filepath.Join(root, "home"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "run"),
		"XDG_STATE_HOME":  filepath.Join(root, "state"),
	}
	for _, dir := range []string{dirs["HOME"], filepath.Join(dirs["XDG_CONFIG_HOME"], "ataecina"), dirs["XDG_RUNTIME_DIR"], filepath.Join(root, "fakes")} {
		mustPrivateDir(t, dir)
	}
	if err := os.WriteFile(filepath.Join(dirs["XDG_CONFIG_HOME"], "ataecina", "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, answer := range fakes {
		if err := os.WriteFile(filepath.Join(root, "fakes", name+".json"), []byte(answer), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"PATH=/usr/bin:/bin", "ATAECINA_E2E_FAKES=" + filepath.Join(root, "fakes")}
	for key, value := range dirs {
		env = append(env, key+"="+value)
	}
	return &tree{t: t, root: root, env: env}
}

func mustPrivateDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}

func (tr *tree) run(bin string, args ...string) (int, string, string) {
	tr.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = tr.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		tr.t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

const config = `{"version": 1, "interval_minutes": 5, "subscriptions": [
  {"id": "claude", "source": "e2e-fake", "plan": "pro"},
  {"id": "minimax", "source": "e2e-fake"},
  {"id": "openrouter", "source": "e2e-fake"},
  {"id": "zai", "source": "zai-api"}
]}`

var fakes = map[string]string{
	"claude":     `{"meters": [{"window_minutes": 300, "used_percent": 23.5, "resets_in_minutes": 130}, {"window_minutes": 10080, "used_percent": 41.2, "resets_in_minutes": 6000}]}`,
	"minimax":    `{"failure": "CONTRACT_CHANGED"}`,
	"openrouter": `{"balance": {"remaining": "8.42", "spent": "11.58", "currency": "USD"}}`,
}

func TestCollectAndShowTheSnapshot(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, config, fakes)

	if code, out, _ := tr.run(bin, "status"); code != 0 || !strings.Contains(out, "No snapshot yet") {
		t.Fatalf("status before collect = %d %q", code, out)
	}
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	info, err := os.Stat(filepath.Join(tr.root, "run", "ataecina", "snapshot.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot = %v", err)
	}

	code, out, stderr := tr.run(bin, "status", "--json")
	if code != 0 {
		t.Fatalf("status --json = %d: %s", code, stderr)
	}
	var doc struct {
		Data struct {
			Subscriptions []struct {
				ID      string `json:"id"`
				Plan    string `json:"plan"`
				Outcome string `json:"outcome"`
				Source  string `json:"source"`
				Balance *struct {
					Remaining struct {
						Amount   float64 `json:"amount"`
						Currency string  `json:"currency"`
					} `json:"remaining"`
				} `json:"balance"`
			} `json:"subscriptions"`
		} `json:"data"`
		Meta struct {
			Version          string   `json:"version"`
			WorstUsedPercent *float64 `json:"worst_used_percent"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]string{}
	for _, s := range doc.Data.Subscriptions {
		outcomes[s.ID] = s.Outcome
	}
	want := map[string]string{"claude": "OK", "minimax": "BROKEN", "openrouter": "OK", "zai": "UNSUPPORTED"}
	for id, outcome := range want {
		if outcomes[id] != outcome {
			t.Fatalf("%s = %s, want %s", id, outcomes[id], outcome)
		}
	}
	if doc.Data.Subscriptions[0].Plan != "PRO" || doc.Data.Subscriptions[0].Source != "E2E_FAKE" {
		t.Fatalf("claude = %+v", doc.Data.Subscriptions[0])
	}
	if b := doc.Data.Subscriptions[2].Balance; b == nil || b.Remaining.Amount != 8.42 || b.Remaining.Currency != "USD" {
		t.Fatalf("openrouter balance = %+v", b)
	}
	if doc.Meta.WorstUsedPercent == nil || *doc.Meta.WorstUsedPercent != 41.2 || doc.Meta.Version == "" {
		t.Fatalf("meta = %+v", doc.Meta)
	}

	code, table, _ := tr.run(bin)
	for _, fragment := range []string{"5h 23.5% ↻ 2h10m", "week 41.2%", "broken", "$8.42 left", "unsupported"} {
		if code != 0 || !strings.Contains(table, fragment) {
			t.Fatalf("table lacks %q:\n%s", fragment, table)
		}
	}
	if strings.Contains(table, "\x1b[") {
		t.Fatal("no colours when stdout is not a terminal")
	}
}

func TestLooseConfigurationIsRefused(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, config, fakes)
	if err := os.Chmod(filepath.Join(tr.root, "config", "ataecina", "config.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := tr.run(bin, "collect")
	if code != 1 || !strings.Contains(stderr, "chmod 600") {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
}

func TestProblemsGoToStderr(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, config, fakes)
	if err := os.WriteFile(filepath.Join(tr.root, "run", "ataecina"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := tr.run(bin, "status", "--json")
	var problem map[string]string
	if code != 1 || stdout != "" || json.Unmarshal([]byte(stderr), &problem) != nil || !strings.HasPrefix(problem["type"], "urn:ataecina:problem:") {
		t.Fatalf("status = %d %q %q", code, stdout, stderr)
	}
	if code, _, _ := tr.run(bin, "dance"); code != 2 {
		t.Fatalf("unknown command = %d", code)
	}
}

func TestTheProductionBinaryHasNoFakeSource(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, config, fakes)
	code, _, stderr := tr.run(bin, "collect")
	if code != 1 || !strings.Contains(stderr, `unknown source "e2e-fake"`) {
		t.Fatalf("production collect = %d: %s", code, stderr)
	}
}
