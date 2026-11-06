//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorPointsAtWhatIsMissing(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [
	  {"id": "zai", "source": "zai-api"},
	  {"id": "zen", "source": "opencode-zen"},
	  {"id": "codex", "source": "codex-logs", "app_server": true}
	]}`)
	w.vars["PATH"] = filepath.Join(w.root, "empty-bin")
	code, out, _ := w.run("doctor")
	for _, want := range []string{
		"✓ configuration",
		"! keys           no keys.json yet",
		"! keyring        no session bus here",
		"! zai            source zai-api is not available in this version",
		"! zen            opencode is not on PATH",
		"! codex          codex is not on PATH",
		"! snapshot       none yet",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if code != exitOK {
		t.Fatalf("warnings alone exit 0, got %d", code)
	}

	if code, _, _ := w.runWithStdin("", "init", "--unlocker", "passphrase", "--passphrase-fd", passphraseFD(t, "a synthetic passphrase")); code != exitOK {
		t.Fatal("init")
	}
	w.collect()
	_, out, _ = w.run("doctor")
	for _, want := range []string{"✓ unlockers      passphrase", "✓ keys", "(0 sealed)", "✓ snapshot       taken 0s ago"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
}

func TestDoctorFailsOnLooseFiles(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": []}`)
	if err := os.Chmod(filepath.Join(w.root, "config", "ataecina", "config.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := w.run("doctor")
	if code != exitFailure || !strings.Contains(out, "✗ configuration") || !strings.Contains(out, "chmod 600") {
		t.Fatalf("doctor = %d\n%s", code, out)
	}
	if err := os.RemoveAll(filepath.Join(w.root, "config")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := w.run("doctor"); code != exitFailure || !strings.Contains(out, "run `ataecina init`") {
		t.Fatalf("no configuration = %d\n%s", code, out)
	}
	if code, _, _ := w.run("doctor", "extra"); code != exitUsage {
		t.Fatal("doctor takes no argument")
	}
	delete(w.vars, "XDG_RUNTIME_DIR")
	if code, out, _ := w.run("doctor"); code != exitFailure || !strings.Contains(out, "✗ snapshot") {
		t.Fatalf("no runtime directory = %d\n%s", code, out)
	}
	delete(w.vars, "HOME")
	if code, _, _ := w.run("doctor"); code != exitFailure {
		t.Fatal("no HOME")
	}
}
