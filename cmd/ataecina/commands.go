package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/journal"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/snapshotfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/statusjson"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
	"codeberg.org/simplicer/ataecina/internal/platform/logging"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2

	collectionDeadline = 20 * time.Second
)

// environment is everything a command may touch, so tests can replace it.
type environment struct {
	getenv          func(string) string
	stdin           io.Reader
	stdout          io.Writer
	stderr          io.Writer
	terminal        bool
	stdinIsTerminal bool
	clock           domain.Clock
	sources         sourceRegistry
	// kdfIterations lowers the PBKDF2 cost in tests; 0 means production.
	kdfIterations int
}

const usage = `Usage: ataecina [command] [options]

Commands:
  status [--json]   show the snapshot (the default; no network, no keys)
  collect           read every configured subscription now and keep the snapshot
  hook claude-statusline [--print] [--subscription <id>]
                    record the Claude limits Claude Code passes on stdin to its
                    status line; --print writes a one-line summary
  balance declare <subscription> (--top-up <amount> | --balance <amount>) [--currency USD]
                    record a top-up or the current balance of OpenCode Zen
  init [--unlocker secret-service|passphrase]
                    create ~/.config/ataecina with safe permissions and the master key
  keys set <name> | list | forget <name>
                    seal a provider key read from stdin without echo, list or forget keys
  unlockers list | add <kind> | remove <kind>
                    manage how the master key is held (secret-service, passphrase, systemd-creds)
  version           print the version
  help              show this help

Commands that unlock the master key accept --passphrase-fd <n> to read the
passphrase from a file descriptor instead of the terminal.

Configuration: $XDG_CONFIG_HOME/ataecina/config.json (see etc/ataecina.example.json).
`

func run(ctx context.Context, args []string, env environment) int {
	command, rest := "status", args
	if len(args) > 0 {
		switch {
		case args[0] == "--version":
			command, rest = "version", args[1:]
		case args[0] == "--help" || args[0] == "-h":
			command, rest = "help", args[1:]
		case !strings.HasPrefix(args[0], "-"):
			command, rest = args[0], args[1:]
		}
	}
	switch command {
	case "status":
		return status(rest, env)
	case "collect":
		return collect(ctx, rest, env)
	case "hook":
		return hook(rest, env)
	case "balance":
		return balance(ctx, rest, env)
	case "init":
		return initialise(ctx, rest, env)
	case "keys":
		return keysCommand(ctx, rest, env)
	case "unlockers":
		return unlockersCommand(ctx, rest, env)
	case "version":
		return printVersion(env)
	case "help":
		fmt.Fprint(env.stdout, usage)
		return exitOK
	}
	fmt.Fprintf(env.stderr, "ataecina: unknown command %q\n\n%s", command, usage)
	return exitUsage
}

func printVersion(env environment) int {
	fmt.Fprintf(env.stdout, "ataecina %s\n", version)
	return exitOK
}

func newFlags(name string, env environment) *flag.FlagSet {
	flags := flag.NewFlagSet("ataecina "+name, flag.ContinueOnError)
	flags.SetOutput(env.stderr)
	return flags
}

// status prints the snapshot; it never collects, unseals or connects.
func status(args []string, env environment) int {
	flags := newFlags("status", env)
	asJSON := flags.Bool("json", false, "print the public JSON contract")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	fail := func(slug, title string, err error) int {
		if *asJSON {
			_ = statusjson.WriteProblem(env.stderr, statusjson.NewProblem(slug, title, err.Error()))
		} else {
			fmt.Fprintf(env.stderr, "ataecina: %s: %v\n", strings.ToLower(title), err)
		}
		return exitFailure
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return fail("environment", "The environment is incomplete", err)
	}
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return fail("environment", "The environment is incomplete", err)
	}
	report, err := application.NewReportSnapshot(snapshotfile.NewStore(runtimeDir), env.clock).Report()
	if err != nil {
		return fail("snapshot-unreadable", "The snapshot cannot be read", err)
	}
	if *asJSON {
		err = statusjson.Write(env.stdout, report, statusjson.Meta{Version: version, GeneratedAt: env.clock.Now()})
	} else {
		err = terminal.Render(env.stdout, report, env.clock.Now(), env.terminal && env.getenv("NO_COLOR") == "")
	}
	if err != nil {
		return exitFailure
	}
	return exitOK
}

// collect reads every subscription once and keeps the snapshot.
func collect(ctx context.Context, args []string, env environment) int {
	flags := newFlags("collect", env)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	fail := func(err error) int {
		fmt.Fprintf(env.stderr, "ataecina: collect: %v\n", err)
		if errors.Is(err, configfile.ErrMissing) {
			fmt.Fprintln(env.stderr, "ataecina: create it with mode 0600 in a 0700 directory; see etc/ataecina.example.json")
		}
		return exitFailure
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return fail(err)
	}
	settings, err := configfile.Load(paths.ConfigFile(), env.sources.catalog)
	if err != nil {
		return fail(err)
	}
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return fail(err)
	}
	if err := privatefs.EnsureDir(paths.StateDir()); err != nil {
		return fail(err)
	}
	logger, logFile, err := logging.Open(env.stderr, paths.LogFile(), slog.LevelInfo)
	if err != nil {
		return fail(err)
	}
	defer logFile.Close()

	store := snapshotfile.NewStore(runtimeDir)
	collector := application.NewCollectReadings(
		settings.DomainSubscriptions(),
		application.CollectionPolicy{Interval: settings.Interval, Deadline: collectionDeadline},
		application.CollectReadingsPorts{
			Sources:  env.sources.build(settings, paths, env),
			Previous: store,
			Store:    store,
			Journal:  journal.New(logger),
			Clock:    env.clock,
		},
	)
	if _, err := collector.Collect(ctx); err != nil {
		logger.Error("collection failed", slog.String("error", err.Error()))
		return exitFailure
	}
	return exitOK
}
