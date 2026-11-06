//go:build integration

package keyfile_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/keyfile"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

var setAt = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func TestTheKeyFileRoundTripsPrivately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "ataecina", "keys.json")
	store := keyfile.NewStore(path)
	if _, err := store.Load(); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("before init = %v", err)
	}
	file := domain.NewKeyFile(domain.NewWrappedMasterKey(domain.Passphrase, []byte(`{"kdf":"pbkdf2-sha256"}`)))
	file.PutKey(domain.NewSealedKey("zai", []byte{1, 2, 3}, []byte{4, 5, 6}, setAt))
	if err := store.Save(file); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
		t.Fatalf("modes = %04o %04o", info.Mode().Perm(), dir.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	for _, fragment := range []string{`"version": 1`, `"master_key"`, `"passphrase": {`, `"zai": {`, `"nonce": "AQID"`, `"set_at": "2026-11-06T19:40:00Z"`} {
		if !bytes.Contains(data, []byte(fragment)) {
			t.Fatalf("missing %s in\n%s", fragment, data)
		}
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if key, ok := loaded.Key("zai"); !ok || key.Sealed()[2] != 6 || len(loaded.Unlockers()) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestBrokenOrLooseKeyFilesAreRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ataecina")
	if err := privatefs.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "keys.json")
	store := keyfile.NewStore(path)
	for name, content := range map[string]string{
		"not json":       `{`,
		"other version":  `{"version": 2}`,
		"no unlocker":    `{"version": 1, "master_key": {}, "keys": {}}`,
		"bad unlocker":   `{"version": 1, "master_key": {"kwallet": {}}, "keys": {}}`,
		"bad key name":   `{"version": 1, "master_key": {"passphrase": {}}, "keys": {"Zai": {"nonce": "", "sealed": ""}}}`,
		"bad nonce":      `{"version": 1, "master_key": {"passphrase": {}}, "keys": {"zai": {"nonce": "!", "sealed": ""}}}`,
		"bad ciphertext": `{"version": 1, "master_key": {"passphrase": {}}, "keys": {"zai": {"nonce": "", "sealed": "!"}}}`,
	} {
		if err := privatefs.WriteFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("0644 keys.json = %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("0755 directory = %v", err)
	}
	broken := domain.NewKeyFile(domain.NewWrappedMasterKey(domain.Passphrase, []byte("not json")))
	if err := keyfile.NewStore(filepath.Join(t.TempDir(), "k", "keys.json")).Save(broken); err == nil {
		t.Fatal("an unlocker record that is not JSON must be refused")
	}
}
