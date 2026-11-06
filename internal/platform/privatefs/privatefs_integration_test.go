//go:build integration

package privatefs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

func TestEnsureDirCreatesAPrivateTree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "ataecina")
	if err := privatefs.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %04o", info.Mode().Perm())
	}
	if err := privatefs.EnsureDir(dir); err != nil {
		t.Fatalf("second EnsureDir = %v", err)
	}
}

func TestOpenDirectoriesAreRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := privatefs.EnsureDir(dir); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("EnsureDir(0755) = %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := privatefs.RequireDir(file); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("RequireDir(file) = %v", err)
	}
	if err := privatefs.RequireDir(filepath.Join(dir, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RequireDir(missing) = %v", err)
	}
	if err := privatefs.EnsureDir(filepath.Join(file, "below")); err == nil {
		t.Fatal("EnsureDir below a file must fail")
	}
}

func TestWriteThenReadAPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := privatefs.WriteFile(path, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := privatefs.WriteFile(path, []byte(`{"a":2}`)); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o", info.Mode().Perm())
	}
	data, err := privatefs.ReadFile(path, 1024)
	if err != nil || string(data) != `{"a":2}` {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %d entries", len(entries))
	}
}

func TestReadRefusesLooseLargeOrIndirectFiles(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "loose")
	if err := os.WriteFile(loose, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := privatefs.ReadFile(loose, 10); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("ReadFile(0644) = %v", err)
	}
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, make([]byte, 11), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := privatefs.ReadFile(large, 10); !errors.Is(err, privatefs.ErrTooLarge) {
		t.Fatalf("ReadFile(large) = %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(large, link); err != nil {
		t.Fatal(err)
	}
	if _, err := privatefs.ReadFile(link, 100); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("ReadFile(symlink) = %v", err)
	}
	if _, err := privatefs.ReadFile(dir, 100); !errors.Is(err, privatefs.ErrNotPrivate) {
		t.Fatalf("ReadFile(dir) = %v", err)
	}
	if _, err := privatefs.ReadFile(filepath.Join(dir, "missing"), 10); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(missing) = %v", err)
	}
}

func TestWriteIntoAMissingDirectoryFails(t *testing.T) {
	if err := privatefs.WriteFile(filepath.Join(t.TempDir(), "missing", "f"), []byte("x")); err == nil {
		t.Fatal("WriteFile into a missing directory must fail")
	}
}
