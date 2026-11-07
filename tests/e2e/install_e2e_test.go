//go:build e2e

package e2e

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installFiles runs bin/install-files with a throw-away DESTDIR and HOME.
func installFiles(t *testing.T, destdir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("../../bin/install-files", append(args, "--destdir", destdir)...)
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir()}
	out, err := command.CombinedOutput()
	return string(out), err
}

func TestTheUserInstallationPlacesEverythingUnderThePrefixAndRemovesIt(t *testing.T) {
	binary(t, "ATAECINA_PRODUCTION_BINARY")
	destdir := t.TempDir()
	const prefix = "/home/someone/.local"
	units := prefix + "/share/systemd/user"
	if out, err := installFiles(t, destdir, "--prefix", prefix, "--systemd-user-dir", units); err != nil {
		t.Fatalf("install-files: %v\n%s", err, out)
	}

	modes := map[string]fs.FileMode{
		prefix + "/bin/ataecina":                                                       0o755,
		units + "/ataecina-collect.service":                                            0o644,
		units + "/ataecina-collect.timer":                                              0o644,
		prefix + "/share/icons/hicolor/scalable/apps/ataecina.svg":                     0o644,
		prefix + "/share/plasma/plasmoids/org.simplicer.ataecina/metadata.json":        0o644,
		prefix + "/share/plasma/plasmoids/org.simplicer.ataecina/contents/ui/main.qml": 0o644,
		prefix + "/share/plasma/plasmoids/org.simplicer.ataecina/contents/locale/es/LC_MESSAGES/plasma_applet_org.simplicer.ataecina.mo": 0o644,
		prefix + "/share/applications/org.simplicer.ataecina.desktop":                                                                    0o644,
		prefix + "/share/doc/ataecina/README.md":                                                                                         0o644,
		prefix + "/share/doc/ataecina/LICENSE":                                                                                           0o644,
		prefix + "/share/doc/ataecina/ataecina.example.json":                                                                             0o644,
	}
	for path, mode := range modes {
		info, err := os.Stat(filepath.Join(destdir, path))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("%s has mode %v, want %v", path, info.Mode().Perm(), mode)
		}
	}
	if _, err := os.Stat(filepath.Join(destdir, prefix, "share/plasma/plasmoids/org.simplicer.ataecina/translate")); err == nil {
		t.Fatal("the translation sources were installed")
	}

	unit, err := os.ReadFile(filepath.Join(destdir, units, "ataecina-collect.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unit), "\nExecStart="+prefix+"/bin/ataecina collect\n") || strings.Contains(string(unit), "@BINDIR@") {
		t.Fatalf("the unit does not run the installed binary:\n%s", unit)
	}

	launcher, err := os.ReadFile(filepath.Join(destdir, prefix, "share/applications/org.simplicer.ataecina.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"\nExec=" + prefix + "/bin/ataecina tray\n", "\nTryExec=" + prefix + "/bin/ataecina\n"} {
		if !strings.Contains(string(launcher), line) {
			t.Fatalf("the launcher lacks %q:\n%s", line, launcher)
		}
	}
	if validate, err := exec.LookPath("desktop-file-validate"); err == nil {
		if out, err := exec.Command(validate, filepath.Join(destdir, prefix, "share/applications/org.simplicer.ataecina.desktop")).CombinedOutput(); err != nil {
			t.Fatalf("desktop-file-validate: %v\n%s", err, out)
		}
	}

	version, err := exec.Command(filepath.Join(destdir, prefix, "bin/ataecina"), "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), strings.TrimSpace(string(want))) {
		t.Fatalf("installed binary reports %q", version)
	}

	if out, err := installFiles(t, destdir, "--uninstall", "--prefix", prefix, "--systemd-user-dir", units); err != nil {
		t.Fatalf("install-files --uninstall: %v\n%s", err, out)
	}
	err = filepath.WalkDir(destdir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			t.Errorf("left behind after uninstall: %s", strings.TrimPrefix(path, destdir))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheInstallationRefusesARelativePrefix(t *testing.T) {
	if out, err := installFiles(t, t.TempDir(), "--prefix", "usr"); err == nil {
		t.Fatalf("install-files accepted a relative prefix:\n%s", out)
	}
}
