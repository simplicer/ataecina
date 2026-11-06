package xdg_test

import (
	"errors"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDefaultsFollowTheSpecification(t *testing.T) {
	paths, err := xdg.Resolve(environment(map[string]string{"HOME": "/home/owner", "XDG_RUNTIME_DIR": "/run/user/1000"}))
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		paths.Home():           "/home/owner",
		paths.ConfigDir():      "/home/owner/.config/ataecina",
		paths.ConfigFile():     "/home/owner/.config/ataecina/config.json",
		paths.StateDir():       "/home/owner/.local/state/ataecina",
		paths.LogFile():        "/home/owner/.local/state/ataecina/ataecina.log",
		paths.KeysFile():       "/home/owner/.config/ataecina/keys.json",
		paths.SystemdUserDir(): "/home/owner/.config/systemd/user",
	}
	for got, want := range checks {
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	runtime, err := paths.RuntimeDir()
	if err != nil || runtime != "/run/user/1000/ataecina" {
		t.Fatalf("RuntimeDir() = %q, %v", runtime, err)
	}
}

func TestExplicitVariablesWinAndRelativeOnesAreIgnored(t *testing.T) {
	paths, err := xdg.Resolve(environment(map[string]string{
		"HOME":            "/home/owner",
		"XDG_CONFIG_HOME": "/tmp/cfg/",
		"XDG_STATE_HOME":  "relative/state",
		"XDG_RUNTIME_DIR": "relative/run",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if paths.ConfigDir() != "/tmp/cfg/ataecina" {
		t.Fatalf("ConfigDir() = %q", paths.ConfigDir())
	}
	if paths.StateDir() != "/home/owner/.local/state/ataecina" {
		t.Fatalf("StateDir() = %q", paths.StateDir())
	}
	if _, err := paths.RuntimeDir(); !errors.Is(err, xdg.ErrNoRuntimeDir) {
		t.Fatalf("RuntimeDir() = %v", err)
	}
}

func TestHomeIsRequired(t *testing.T) {
	for _, home := range []string{"", "relative"} {
		if _, err := xdg.Resolve(environment(map[string]string{"HOME": home})); !errors.Is(err, xdg.ErrNoHome) {
			t.Fatalf("Resolve(HOME=%q) = %v", home, err)
		}
	}
}
