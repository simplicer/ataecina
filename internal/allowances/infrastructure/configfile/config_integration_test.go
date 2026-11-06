//go:build integration

package configfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

func privateConfig(t *testing.T, content []byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ataecina")
	if err := privatefs.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := privatefs.WriteFile(path, content); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheShippedExampleIsValid(t *testing.T) {
	example, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "etc", "ataecina.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	settings, err := configfile.Load(privateConfig(t, example), catalog.Standard())
	if err != nil {
		t.Fatalf("the example configuration is invalid: %v", err)
	}
	if len(settings.Subscriptions) == 0 {
		t.Fatal("the example lists no subscription")
	}
}

func TestLooseOrMissingConfigurationIsRefused(t *testing.T) {
	path := privateConfig(t, []byte(`{"version": 1}`))
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := configfile.Load(path, catalog.Standard()); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("0640 config = %v", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := configfile.Load(path, catalog.Standard()); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("0755 directory = %v", err)
	}
	if _, err := configfile.Load(filepath.Join(t.TempDir(), "absent", "config.json"), catalog.Standard()); !errors.Is(err, configfile.ErrMissing) {
		t.Fatalf("missing directory = %v", err)
	}
	empty := filepath.Join(t.TempDir(), "ataecina")
	if err := privatefs.EnsureDir(empty); err != nil {
		t.Fatal(err)
	}
	if _, err := configfile.Load(filepath.Join(empty, "config.json"), catalog.Standard()); !errors.Is(err, configfile.ErrMissing) {
		t.Fatalf("missing file = %v", err)
	}
}
