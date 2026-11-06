// Command ataecina tells its owner how much is left of each AI
// subscription and when each quota window is reborn.
package main

import (
	"context"
	"os"
	"syscall"

	"codeberg.org/simplicer/ataecina/internal/platform/clock"
)

// version is set at build time with -ldflags "-X main.version=$(VERSION)".
var version = "0.0.0-dev"

func main() {
	// Every file Ataecina creates is private (AGENTS.md, security rules).
	syscall.Umask(0o077)
	os.Exit(run(context.Background(), os.Args[1:], environment{
		getenv:          os.Getenv,
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		terminal:        isTerminal(os.Stdout),
		stdinIsTerminal: isTerminal(os.Stdin),
		clock:           clock.System{},
		sources:         buildSources(),
	}))
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
