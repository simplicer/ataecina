//go:build acceptance

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// passphraseFD returns a descriptor that yields the given lines, as
// `--passphrase-fd` reads them.
func passphraseFD(t *testing.T, lines ...string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if _, err := w.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	return strconv.Itoa(int(r.Fd()))
}

func (w *world) runWithStdin(input string, args ...string) (int, string, string) {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	env := w.env
	env.stdin, env.stdout, env.stderr = strings.NewReader(input), &stdout, &stderr
	env.kdfIterations = 1000
	code := run(context.Background(), args, env)
	return code, stdout.String(), stderr.String()
}

func TestKeysAreSealedUnderAPassphrase(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": []}`)
	const phrase = "a synthetic passphrase"
	if code, out, stderr := w.runWithStdin("", "init", "--unlocker", "passphrase", "--passphrase-fd", passphraseFD(t, phrase)); code != exitOK || !strings.Contains(out, "held by passphrase") {
		t.Fatalf("init = %d %q %q", code, out, stderr)
	}
	if code, out, _ := w.runWithStdin("", "init", "--unlocker", "passphrase"); code != exitOK || !strings.Contains(out, "nothing changed") {
		t.Fatalf("init twice = %d %q", code, out)
	}
	if code, out, stderr := w.runWithStdin("sk-synthetic-zai\n", "keys", "set", "zai", "--passphrase-fd", passphraseFD(t, phrase)); code != exitOK || out != "sealed key zai\n" {
		t.Fatalf("keys set = %d %q %q", code, out, stderr)
	}
	keysJSON, _ := os.ReadFile(filepath.Join(w.root, "config", "ataecina", "keys.json"))
	if bytes.Contains(keysJSON, []byte("sk-synthetic")) || !bytes.Contains(keysJSON, []byte(`"zai"`)) {
		t.Fatalf("keys.json:\n%s", keysJSON)
	}
	if code, out, _ := w.runWithStdin("", "keys", "list"); code != exitOK || !strings.HasPrefix(out, "zai") || strings.Contains(out, "sk-") {
		t.Fatalf("keys list = %d %q", code, out)
	}
	if code, _, stderr := w.runWithStdin("sk-other\n", "keys", "set", "zai", "--passphrase-fd", passphraseFD(t, "a wrong passphrase")); code != exitFailure || !strings.Contains(stderr, "wrong passphrase") {
		t.Fatalf("wrong passphrase = %d %q", code, stderr)
	}
	if code, out, _ := w.runWithStdin("", "unlockers", "list"); code != exitOK || out != "passphrase\n" {
		t.Fatalf("unlockers list = %d %q", code, out)
	}
	if code, _, stderr := w.runWithStdin("", "unlockers", "remove", "passphrase"); code != exitFailure || !strings.Contains(stderr, "last unlocker") {
		t.Fatalf("remove the last unlocker = %d %q", code, stderr)
	}
	if code, out, _ := w.runWithStdin("", "keys", "forget", "zai"); code != exitOK || out != "forgot key zai\n" {
		t.Fatalf("keys forget = %d %q", code, out)
	}
}

func TestKeysAreValidatedBeforeAnythingIsUnlocked(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": []}`)
	cases := []struct {
		input string
		args  []string
		code  int
	}{
		{"", []string{"keys"}, exitUsage},
		{"", []string{"keys", "rotate"}, exitUsage},
		{"", []string{"keys", "set"}, exitUsage},
		{"", []string{"keys", "set", "Bad Name"}, exitUsage},
		{"", []string{"keys", "list", "extra"}, exitUsage},
		{"", []string{"keys", "list"}, exitFailure},
		{"k\n", []string{"keys", "set", "zai"}, exitFailure},
		{"", []string{"unlockers"}, exitUsage},
		{"", []string{"unlockers", "add", "kwallet"}, exitUsage},
		{"", []string{"unlockers", "list", "extra"}, exitUsage},
		{"", []string{"unlockers", "list"}, exitFailure},
		{"", []string{"unlockers", "add", "passphrase"}, exitFailure},
		{"", []string{"init", "extra"}, exitUsage},
		{"", []string{"keys", "list", "--passphrase-fd", "999"}, exitFailure},
		{"", []string{"init", "--unlocker", "kwallet"}, exitFailure},
		{"", []string{"init", "--unlocker", "secret-service"}, exitFailure},
	}
	for _, c := range cases {
		if code, _, _ := w.runWithStdin(c.input, c.args...); code != c.code {
			t.Fatalf("%v = %d, want %d", c.args, code, c.code)
		}
	}
	if code, _, _ := w.runWithStdin("", "init", "--passphrase-fd", passphraseFD(t, "a synthetic passphrase")); code != exitOK {
		t.Fatal("init with the default unlocker")
	}
	for _, input := range []string{"", "\n", "with space\n", "tab\tkey\n", strings.Repeat("k", 5000)} {
		if code, _, _ := w.runWithStdin(input, "keys", "set", "zai", "--passphrase-fd", passphraseFD(t, "a synthetic passphrase")); code != exitFailure {
			t.Fatalf("key %.20q accepted", input)
		}
	}
	delete(w.vars, "HOME")
	for _, args := range [][]string{{"init"}, {"keys", "list"}, {"unlockers", "list"}} {
		if code, _, _ := w.runWithStdin("", args...); code != exitFailure {
			t.Fatalf("%v without HOME = %d", args, code)
		}
	}
}
