//go:build e2e

package e2e

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheSystemdUnitsAreValid checks the shipped units with
// `systemd-analyze --user verify`, pointing them at the built binary.
// Nothing is installed.
func TestTheSystemdUnitsAreValid(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	analyze, err := exec.LookPath("systemd-analyze")
	if err != nil {
		t.Skip("systemd-analyze is not installed")
	}
	dir := t.TempDir()
	service, err := os.ReadFile(filepath.Join("..", "..", "infra", "systemd", "ataecina-collect.service.in"))
	if err != nil {
		t.Fatal(err)
	}
	timer, err := os.ReadFile(filepath.Join("..", "..", "infra", "systemd", "ataecina-collect.timer"))
	if err != nil {
		t.Fatal(err)
	}
	unit := strings.ReplaceAll(string(service), "@BINDIR@", filepath.Dir(bin))
	if strings.Contains(unit, "@BINDIR@") || !strings.Contains(unit, "ExecStart="+filepath.Dir(bin)+"/ataecina collect") {
		t.Fatal("a placeholder is left in the service")
	}
	for name, content := range map[string]string{"ataecina-collect.service": unit, "ataecina-collect.timer": string(timer)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command(analyze, "--user", "verify", filepath.Join(dir, "ataecina-collect.service"), filepath.Join(dir, "ataecina-collect.timer")).CombinedOutput()
	if err != nil || strings.Contains(string(out), "ataecina-collect") {
		t.Fatalf("systemd-analyze verify: %v\n%s", err, out)
	}
	for _, hardening := range []string{"NoNewPrivileges=yes", "ProtectSystem=strict", "ProtectHome=read-only", "PrivateTmp=yes", "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6", "SystemCallFilter=@system-service", "UMask=0077", "StateDirectoryMode=0700", "RuntimeDirectoryPreserve=yes"} {
		if !strings.Contains(unit, hardening) {
			t.Fatalf("the service lacks %s", hardening)
		}
	}
}

func TestASystemdCredentialOpensTheKeysInsideTheUnit(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "vercel", "source": "vercel-api"}]}`, nil)
	origin, _ := fakeProviders(t)
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", origin)
	tools := filepath.Join(tr.root, "tools")
	mustPrivateDir(t, tools)
	script, err := os.ReadFile("testdata/fake-systemd-creds")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "systemd-creds"), script, 0o700); err != nil {
		t.Fatal(err)
	}
	tr.setenv("PATH", tools+":/usr/bin:/bin")
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatalf("init = %d %s", code, stderr)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "sk-synthetic-vercel\n", phrase, "keys", "set", "vercel"); code != 0 {
		t.Fatalf("keys set = %d %s", code, stderr)
	}
	code, out, stderr := tr.runWithPassphrase(bin, "", phrase, "unlockers", "add", "systemd-creds")
	if code != 0 || !strings.Contains(out, "daemon-reload") {
		t.Fatalf("unlockers add = %d %q %q", code, out, stderr)
	}
	dropIn := filepath.Join(tr.root, "config", "systemd", "user", "ataecina-collect.service.d", "master-key.conf")
	if data, err := os.ReadFile(dropIn); err != nil || !strings.Contains(string(data), "LoadCredentialEncrypted=ataecina.master-key:") {
		t.Fatalf("drop-in = %v", err)
	}

	// What systemd does for the unit: decrypt the credential into
	// $CREDENTIALS_DIRECTORY. The collector then needs no passphrase.
	encoded, err := os.ReadFile(filepath.Join(tr.root, "config", "ataecina", "master-key.cred"))
	if err != nil {
		t.Fatal(err)
	}
	master, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	credentials := filepath.Join(tr.root, "credentials")
	mustPrivateDir(t, credentials)
	if err := os.WriteFile(filepath.Join(credentials, "ataecina.master-key"), master, 0o400); err != nil {
		t.Fatal(err)
	}
	tr.setenv("PATH", "/usr/bin:/bin")
	tr.setenv("CREDENTIALS_DIRECTORY", credentials)
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d %s", code, stderr)
	}
	if _, status, _ := tr.run(bin, "status", "--json"); !strings.Contains(status, `"outcome": "OK"`) {
		t.Fatalf("status:\n%s", status)
	}
}
