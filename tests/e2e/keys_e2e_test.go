//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWithPassphrase runs the binary with input on stdin and the
// passphrase on file descriptor 3 (`--passphrase-fd 3`).
func (tr *tree) runWithPassphrase(bin, input, phrase string, args ...string) (int, string, string) {
	tr.t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		tr.t.Fatal(err)
	}
	defer r.Close()
	if _, err := w.WriteString(phrase + "\n"); err != nil {
		tr.t.Fatal(err)
	}
	w.Close()
	cmd := exec.Command(bin, append(args, "--passphrase-fd", "3")...)
	cmd.Env = tr.env
	cmd.Stdin = strings.NewReader(input)
	cmd.ExtraFiles = []*os.File{r}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		tr.t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

func TestKeysAreSealedEndToEnd(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": []}`, nil)
	if err := os.RemoveAll(filepath.Join(tr.root, "config", "ataecina")); err != nil {
		t.Fatal(err)
	}
	const phrase = "a synthetic passphrase"
	if code, out, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 || !strings.Contains(out, "held by passphrase") {
		t.Fatalf("init = %d %q %q", code, out, stderr)
	}
	for path, mode := range map[string]os.FileMode{
		filepath.Join(tr.root, "config", "ataecina"):                0o700,
		filepath.Join(tr.root, "config", "ataecina", "config.json"): 0o600,
		filepath.Join(tr.root, "config", "ataecina", "keys.json"):   0o600,
	} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s = %v", path, err)
		}
	}
	if code, out, stderr := tr.runWithPassphrase(bin, "sk-synthetic-openrouter\n", phrase, "keys", "set", "openrouter"); code != 0 || out != "sealed key openrouter\n" {
		t.Fatalf("keys set = %d %q %q", code, out, stderr)
	}
	keysJSON, _ := os.ReadFile(filepath.Join(tr.root, "config", "ataecina", "keys.json"))
	if bytes.Contains(keysJSON, []byte("sk-synthetic")) {
		t.Fatal("keys.json holds a key in clear")
	}
	if code, out, _ := tr.run(bin, "keys", "list"); code != 0 || !strings.HasPrefix(out, "openrouter") {
		t.Fatalf("keys list = %d %q", code, out)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "sk-x\n", "not the passphrase", "keys", "set", "zai"); code != 1 || !strings.Contains(stderr, "wrong passphrase") || strings.Contains(stderr, "sk-x") {
		t.Fatalf("wrong passphrase = %d %q", code, stderr)
	}
}
