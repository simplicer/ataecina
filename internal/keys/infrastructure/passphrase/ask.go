package passphrase

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"strings"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
	"codeberg.org/simplicer/ataecina/internal/platform/terminal"
)

// FromTerminal asks a person on /dev/tty without echo; without a terminal
// the passphrase unlocker is unavailable (a systemd service never prompts).
func FromTerminal() Ask {
	return func(prompt string, confirm bool) (secret.Key, error) {
		first, err := terminal.ReadSecret(prompt)
		if err != nil {
			return secret.Key{}, fmt.Errorf("%w: %w", domain.ErrUnlockerUnavailable, err)
		}
		defer clear(first)
		if confirm {
			second, err := terminal.ReadSecret("Repeat it: ")
			if err != nil {
				return secret.Key{}, fmt.Errorf("%w: %w", domain.ErrUnlockerUnavailable, err)
			}
			defer clear(second)
			if subtle.ConstantTimeCompare(first, second) != 1 {
				return secret.Key{}, ErrMismatch
			}
		}
		return secret.New(first)
	}
}

// FromReader reads one passphrase per line from r (`--passphrase-fd N`),
// for scripts and tests; nothing is echoed because nothing is typed.
func FromReader(r io.Reader) Ask {
	lines := bufio.NewReaderSize(r, 4096)
	return func(string, bool) (secret.Key, error) {
		line, err := lines.ReadString('\n')
		if err != nil && line == "" {
			return secret.Key{}, fmt.Errorf("no passphrase on the descriptor: %w", err)
		}
		raw := []byte(strings.TrimRight(line, "\r\n"))
		defer clear(raw)
		if len(bytes.TrimSpace(raw)) == 0 {
			return secret.Key{}, fmt.Errorf("an empty passphrase")
		}
		return secret.New(raw)
	}
}
