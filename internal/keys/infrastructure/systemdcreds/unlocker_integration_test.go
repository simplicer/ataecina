//go:build integration

package systemdcreds_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/systemdcreds"
)

// fake is tests/e2e/testdata/fake-systemd-creds; the real tool never runs.
func fake(t *testing.T) func() (string, error) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "tests", "e2e", "testdata", "fake-systemd-creds"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_CREDS_JOURNAL", filepath.Join(t.TempDir(), "journal"))
	return func() (string, error) { return path, nil }
}

func paths(t *testing.T) systemdcreds.Paths {
	root := t.TempDir()
	return systemdcreds.Paths{
		CredentialFile: filepath.Join(root, "config", "ataecina", "master-key.cred"),
		DropIn:         filepath.Join(root, "config", "systemd", "user", "ataecina-collect.service.d", "master-key.conf"),
	}
}

func TestTheMasterKeyBecomesACredentialOfTheUnit(t *testing.T) {
	p := paths(t)
	unlocker := systemdcreds.New(fake(t), p)
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	wrapped, err := unlocker.Wrap(context.Background(), master)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped.Kind() != domain.SystemdCreds || bytes.Contains(wrapped.Data(), master.Reveal()) {
		t.Fatalf("record = %s", wrapped.Data())
	}
	if info, err := os.Stat(p.CredentialFile); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credential file = %v", err)
	}
	dropIn, _ := os.ReadFile(p.DropIn)
	if !strings.Contains(string(dropIn), "LoadCredentialEncrypted=ataecina.master-key:"+p.CredentialFile) {
		t.Fatalf("drop-in:\n%s", dropIn)
	}
	opened, err := unlocker.Unwrap(context.Background(), wrapped)
	if err != nil || !bytes.Equal(opened.Reveal(), master.Reveal()) {
		t.Fatalf("decrypt outside the unit = %v", err)
	}

	// Inside the unit, systemd has already decrypted the credential.
	inside := p
	inside.CredentialsDirectory = t.TempDir()
	if err := os.WriteFile(filepath.Join(inside.CredentialsDirectory, systemdcreds.CredentialName), master.Reveal(), 0o400); err != nil {
		t.Fatal(err)
	}
	broken := systemdcreds.New(func() (string, error) { return "", errors.New("not installed") }, inside)
	if opened, err := broken.Unwrap(context.Background(), wrapped); err != nil || !bytes.Equal(opened.Reveal(), master.Reveal()) {
		t.Fatalf("inside the unit = %v", err)
	}

	if err := unlocker.Forget(context.Background(), wrapped); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{p.CredentialFile, p.DropIn} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists", path)
		}
	}
	if err := unlocker.Forget(context.Background(), wrapped); err != nil {
		t.Fatal("forgetting twice is fine")
	}
}

func TestWithoutTheToolTheUnlockerIsUnavailable(t *testing.T) {
	missing := systemdcreds.New(func() (string, error) { return "", errors.New("not installed") }, paths(t))
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	if _, err := missing.Wrap(context.Background(), master); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("Wrap = %v", err)
	}
	wrapped := domain.NewWrappedMasterKey(domain.SystemdCreds, []byte(`{}`))
	if _, err := missing.Unwrap(context.Background(), wrapped); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("Unwrap = %v", err)
	}
	failing := systemdcreds.New(func() (string, error) { return "/bin/false", nil }, paths(t))
	if _, err := failing.Wrap(context.Background(), master); err == nil || !strings.Contains(err.Error(), "systemd-creds encrypt failed") {
		t.Fatalf("a failing tool = %v", err)
	}
	p := paths(t)
	wrong := systemdcreds.New(fake(t), p)
	if _, err := wrong.Wrap(context.Background(), master); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.CredentialFile, []byte("c2hvcnQ=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Unwrap(context.Background(), wrapped); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("a credential that is not a master key = %v", err)
	}
}
