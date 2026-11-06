package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/claudestatusline"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/terminal"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// maxStatusLineBytes caps the status line input read from stdin.
const maxStatusLineBytes = 1 << 20

const hookUsage = "usage: ataecina hook claude-statusline [--print] [--subscription <id>]"

// hook records what Claude Code hands to its status line. It always exits
// 0, holds no key, opens no socket and reads no configuration, so it can
// never break or slow the owner's status line (SPECS §3.1).
func hook(args []string, env environment) int {
	if len(args) == 0 || args[0] != "claude-statusline" {
		fmt.Fprintln(env.stderr, "ataecina: hook: "+hookUsage)
		return exitOK
	}
	flags := newFlags("hook claude-statusline", env)
	printSummary := flags.Bool("print", false, "print a one-line summary for the status line")
	subscription := flags.String("subscription", "claude", "subscription id of this Claude account")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() > 0 {
		fmt.Fprintln(env.stderr, "ataecina: hook: "+hookUsage)
		return exitOK
	}
	reading, err := recordClaudeLimits(*subscription, env)
	if *printSummary {
		fmt.Fprintln(env.stdout, terminal.Summary(reading, env.clock.Now()))
	}
	if err != nil && !errors.Is(err, domain.ErrNoSignal) {
		fmt.Fprintf(env.stderr, "ataecina: hook: %v\n", err)
	}
	return exitOK
}

func recordClaudeLimits(id string, env environment) (domain.Reading, error) {
	subscriptionID, err := domain.NewSubscriptionID(id)
	if err != nil {
		return domain.Reading{}, err
	}
	input, err := io.ReadAll(io.LimitReader(env.stdin, maxStatusLineBytes+1))
	if err != nil {
		return domain.Reading{}, err
	}
	var reported []domain.Meter
	if len(input) <= maxStatusLineBytes {
		// Malformed input records nothing and shows what is already known.
		reported, _ = claudestatusline.DecodeRateLimits(input)
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return domain.Reading{}, err
	}
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return domain.Reading{}, err
	}
	claude := domain.NewSubscription(subscriptionID, "anthropic", claudestatusline.SourceID, domain.FidelityReported, "")
	signals := claudestatusline.NewSignals(filepath.Join(runtimeDir, "signals"))
	return application.NewRecordClaudeLimits(signals, env.clock).Record(claude, reported)
}
