// Package release checks the version scripts of bin/ on a throw-away copy
// of the files they keep in step (AGENTS.md, "Versioning"): VERSION, the
// README, the CHANGELOG, the widget metadata and the Arch PKGBUILD.
package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const repository = "../.."

var versioned = []string{
	"VERSION",
	"README.md",
	"CHANGELOG.md",
	"bin/set-version",
	"bin/check-version",
	"bin/package-version",
	"share/plasma/plasmoids/org.simplicer.ataecina/metadata.json",
	"infra/packaging/arch/PKGBUILD",
}

// copyRepository copies the versioned files into a directory outside any
// git work tree, so check-version skips the history comparison.
func copyRepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range versioned {
		content, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(filepath.Join(dir, args[0]), args[1:]...)
	command.Dir = dir
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir)}
	out, err := command.CombinedOutput()
	return string(out), err
}

func field(t *testing.T, dir, name, pattern string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(pattern).FindStringSubmatch(string(content))
	if match == nil {
		t.Fatalf("%s has no %s", name, pattern)
	}
	return match[1]
}

func TestSetVersionPropagatesEveryVersionForm(t *testing.T) {
	cases := []struct{ version, pkgver string }{
		{"0.13.0-unstable", "0.13.0unstable"},
		{"1.0.0-rc.2", "1.0.0rc2"},
		{"1.2.0-lts", "1.2.0"},
		{"1.3.1", "1.3.1"},
	}
	dir := copyRepository(t)
	for _, c := range cases {
		if out, err := run(t, dir, "bin/set-version", c.version); err != nil {
			t.Fatalf("set-version %s: %v\n%s", c.version, err, out)
		}
		if out, err := run(t, dir, "bin/check-version"); err != nil {
			t.Fatalf("check-version after %s: %v\n%s", c.version, err, out)
		}
		got := map[string]string{
			"VERSION":  strings.TrimSpace(field(t, dir, "VERSION", `(?s)(.*)`)),
			"README":   field(t, dir, "README.md", "\\*\\*Current version:\\*\\* `([^`]+)`"),
			"metadata": field(t, dir, "share/plasma/plasmoids/org.simplicer.ataecina/metadata.json", `"Version": "([^"]+)"`),
			"_tag":     field(t, dir, "infra/packaging/arch/PKGBUILD", `(?m)^_tag=v(.+)$`),
		}
		for where, version := range got {
			if version != c.version {
				t.Fatalf("%s states %q after set-version %s", where, version, c.version)
			}
		}
		if pkgver := field(t, dir, "infra/packaging/arch/PKGBUILD", `(?m)^pkgver=(.+)$`); pkgver != c.pkgver {
			t.Fatalf("pkgver = %q for %s, want %q", pkgver, c.version, c.pkgver)
		}
		if section := field(t, dir, "CHANGELOG.md", `(?m)^## \[(`+regexp.QuoteMeta(c.version)+`)\]`); section != c.version {
			t.Fatalf("CHANGELOG has no section for %s", c.version)
		}
	}
}

func TestCheckVersionCatchesDrift(t *testing.T) {
	for _, drift := range []struct{ name, pattern, replacement string }{
		{"infra/packaging/arch/PKGBUILD", `(?m)^pkgver=.*$`, "pkgver=9.9.9"},
		{"infra/packaging/arch/PKGBUILD", `(?m)^_tag=.*$`, "_tag=v9.9.9"},
		{"share/plasma/plasmoids/org.simplicer.ataecina/metadata.json", `"Version": "[^"]+"`, `"Version": "9.9.9"`},
		{"README.md", "\\*\\*Current version:\\*\\* `[^`]+`", "**Current version:** `9.9.9`"},
	} {
		dir := copyRepository(t)
		path := filepath.Join(dir, drift.name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		changed := regexp.MustCompile(drift.pattern).ReplaceAllString(string(content), drift.replacement)
		if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := run(t, dir, "bin/check-version"); err == nil {
			t.Fatalf("check-version accepted a drifted %s:\n%s", drift.name, out)
		}
	}
}

func TestSetVersionRefusesAStableZeroMajor(t *testing.T) {
	dir := copyRepository(t)
	if out, err := run(t, dir, "bin/set-version", "0.13.0"); err == nil {
		t.Fatalf("set-version accepted 0.13.0:\n%s", out)
	}
}
