//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice/secretservicetest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

// The binary talks to a private dbus-daemon with a fake Secret Service;
// the owner's keyring is never touched.
func TestTheDesktopKeyringHoldsTheMasterKey(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": []}`, nil)
	if err := os.RemoveAll(filepath.Join(tr.root, "config", "ataecina")); err != nil {
		t.Fatal(err)
	}
	address := dbustest.Start(t)
	keyring := secretservicetest.Start(t, address)
	tr.setenv("DBUS_SESSION_BUS_ADDRESS", address)

	if code, out, stderr := tr.run(bin, "init"); code != 0 || !strings.Contains(out, "held by secret-service") {
		t.Fatalf("init = %d %q %q", code, out, stderr)
	}
	if master, ok := keyring.Value(secretservice.Attributes); !ok || len(master) != 32 {
		t.Fatal("the keyring must hold a 32-byte master key")
	}
	if code, out, stderr, _ := tr.runWithInput(bin, "sk-synthetic-vercel\n", "keys", "set", "vercel"); code != 0 || out != "sealed key vercel\n" {
		t.Fatalf("keys set without any prompt = %d %q %q", code, out, stderr)
	}
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "unlockers", "add", "passphrase"); code != 0 {
		t.Fatalf("unlockers add = %d %q", code, stderr)
	}
	if code, out, _ := tr.run(bin, "unlockers", "list"); code != 0 || out != "secret-service\npassphrase\n" {
		t.Fatalf("unlockers list = %d %q", code, out)
	}
	if code, _, stderr := tr.run(bin, "unlockers", "remove", "secret-service"); code != 0 || keyring.Items() != 0 {
		t.Fatalf("unlockers remove = %d %q, %d items left", code, stderr, keyring.Items())
	}
	if code, out, stderr := tr.runWithPassphrase(bin, "sk-synthetic-zai\n", phrase, "keys", "set", "zai"); code != 0 || out != "sealed key zai\n" {
		t.Fatalf("keys set with the passphrase = %d %q %q", code, out, stderr)
	}
	_, out, _ := tr.run(bin, "doctor")
	if !strings.Contains(out, "✓ keyring        org.freedesktop.secrets is provided by") || !strings.Contains(out, "(2 sealed)") {
		t.Fatalf("doctor:\n%s", out)
	}
}
