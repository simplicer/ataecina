//go:build integration

package pathlookup_test

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/platform/pathlookup"
)

func TestOnlyExecutablesInAbsoluteDirectoriesAreFound(t *testing.T) {
	empty, bin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "codex"), []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := pathlookup.Find("codex", "relative/dir:"+empty+":"+bin)()
	if err != nil || found != filepath.Join(bin, "codex") {
		t.Fatalf("Find = %q, %v", found, err)
	}
	if _, err := pathlookup.Find("codex", empty+":relative")(); err == nil {
		t.Fatal("a non-executable file or a relative directory must not be found")
	}
}
