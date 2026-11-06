package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/snapshotfile"
	keysapp "codeberg.org/simplicer/ataecina/internal/keys/application"
	keysdomain "codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice"
	"codeberg.org/simplicer/ataecina/internal/platform/pathlookup"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// report collects doctor's findings: ✓ fine, ! worth a look, ✗ broken.
type report struct {
	env      environment
	problems int
}

func (r *report) ok(topic, format string, args ...any)   { r.line("✓", topic, format, args...) }
func (r *report) warn(topic, format string, args ...any) { r.line("!", topic, format, args...) }
func (r *report) fail(topic, format string, args ...any) {
	r.problems++
	r.line("✗", topic, format, args...)
}

func (r *report) line(mark, topic, format string, args ...any) {
	fmt.Fprintf(r.env.stdout, "%s %-14s %s\n", mark, topic, fmt.Sprintf(format, args...))
}

// doctor checks permissions, configuration, keys, the keyring and the
// sources. It reads only: no network, no unsealing, no prompt.
func doctor(args []string, env environment) int {
	flags := newFlags("doctor", env)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	r := &report{env: env}
	fmt.Fprintf(env.stdout, "ataecina %s\n", version)
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		r.fail("environment", "%v", err)
		return exitFailure
	}
	settings, configured := checkConfiguration(r, paths, env)
	sealed := checkKeys(r, paths)
	checkKeyring(r, env)
	if configured {
		checkSubscriptions(r, settings, sealed, paths, env)
	}
	checkSnapshot(r, paths, env)
	if r.problems > 0 {
		return exitFailure
	}
	return exitOK
}

func checkConfiguration(r *report, paths xdg.Paths, env environment) (configfile.Settings, bool) {
	if err := privatefs.RequireDir(paths.ConfigDir()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			r.fail("configuration", "%s does not exist: run `ataecina init`", paths.ConfigDir())
		} else {
			r.fail("configuration", "%v", err)
		}
		return configfile.Settings{}, false
	}
	settings, err := configfile.Load(paths.ConfigFile(), env.sources.catalog)
	if err != nil {
		r.fail("configuration", "%v", err)
		return configfile.Settings{}, false
	}
	r.ok("configuration", "%s (%d subscriptions, every %s)", paths.ConfigFile(), len(settings.Subscriptions), settings.Interval)
	return settings, true
}

func checkKeys(r *report, paths xdg.Paths) map[string]bool {
	files := keysFiles(paths)
	unlockers, err := keysapp.NewListUnlockers(files).List()
	switch {
	case errors.Is(err, keysdomain.ErrNotInitialized):
		r.warn("keys", "no keys.json yet: run `ataecina init` before `ataecina keys set`")
		return nil
	case err != nil:
		r.fail("keys", "%v", err)
		return nil
	}
	names := make([]string, len(unlockers))
	for i, kind := range unlockers {
		names[i] = string(kind)
	}
	r.ok("unlockers", "%s", strings.Join(names, ", "))
	keys, _ := keysapp.NewListKeys(files).List()
	sealed := map[string]bool{}
	for _, key := range keys {
		sealed[key.Name().String()] = true
	}
	r.ok("keys", "%s (%d sealed)", paths.KeysFile(), len(keys))
	return sealed
}

func checkKeyring(r *report, env environment) {
	address := sessionBusAddress(env)
	if address == "" {
		r.warn("keyring", "no session bus here (SSH or a text console): use the passphrase unlocker")
		return
	}
	desktop := secretservice.New(func() (*dbus.Conn, error) { return dbus.Connect(address) }, false)
	name, pid, err := desktop.Owner()
	if err != nil {
		r.warn("keyring", "nothing owns %s on the session bus: use the passphrase unlocker", secretservice.BusName)
		return
	}
	process := "unknown process"
	if comm, err := os.ReadFile(filepath.Join("/proc", fmt.Sprint(pid), "comm")); err == nil {
		process = strings.TrimSpace(string(comm))
	}
	r.ok("keyring", "%s is provided by %s (pid %d, %s)", secretservice.BusName, process, pid, name)
}

func checkSubscriptions(r *report, settings configfile.Settings, sealed map[string]bool, paths xdg.Paths, env environment) {
	for _, item := range settings.Subscriptions {
		subscription := item.Subscription
		topic := subscription.ID().String()
		entry, _ := env.sources.catalog.Lookup(subscription.Source())
		_, implemented := env.sources.factories[subscription.Source()]
		switch {
		case !implemented:
			r.warn(topic, "source %s is not available in this version (UNSUPPORTED)", subscription.Source())
		case entry.NeedsKey && !sealed[item.Key]:
			r.fail(topic, "no sealed key %q: run `ataecina keys set %s`", item.Key, item.Key)
		default:
			r.ok(topic, "source %s", subscription.Source())
		}
		for _, tool := range toolsOf(subscription.Source().String(), item.AppServer) {
			if _, err := pathlookup.Find(tool, env.getenv("PATH"))(); err != nil {
				r.warn(topic, "%s is not on PATH (%s)", tool, env.getenv("PATH"))
			}
		}
	}
}

// toolsOf lists the official CLIs a source runs.
func toolsOf(source string, appServer bool) []string {
	switch {
	case source == "codex-app-server" || (source == "codex-logs" && appServer):
		return []string{"codex"}
	case source == "opencode-zen":
		return []string{"opencode"}
	}
	return nil
}

func checkSnapshot(r *report, paths xdg.Paths, env environment) {
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		r.fail("snapshot", "%v", err)
		return
	}
	report, err := application.NewReportSnapshot(snapshotfile.NewStore(runtimeDir), env.clock).Report()
	if err != nil {
		r.fail("snapshot", "%v", err)
		return
	}
	takenAt, taken := report.TakenAt()
	if !taken {
		r.warn("snapshot", "none yet: run `ataecina collect`")
		return
	}
	r.ok("snapshot", "taken %s ago", env.clock.Now().Sub(takenAt).Round(time.Second))
}
