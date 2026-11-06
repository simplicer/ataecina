// Package officialcli runs a provider's official CLI once, the way the
// security rules allow (AGENTS.md): the binary found on PATH, a fixed
// argv, no shell, a timeout, stderr discarded and stdout capped. The CLI
// keeps using its own login; Ataecina never sees it.
package officialcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Errors the adapters classify.
var (
	ErrNotFound = errors.New("not on PATH")
	ErrTooLarge = errors.New("answered too much")
	ErrFailed   = errors.New("did not answer")
)

// Command is one fixed invocation.
type Command struct {
	// Find locates the binary (see pathlookup).
	Find func() (string, error)
	Args []string
	// Env is added to the collector's environment, for switches such as
	// GH_NO_UPDATE_NOTIFIER=1.
	Env       []string
	Timeout   time.Duration
	MaxOutput int
}

// Output runs the command and returns its stdout.
func (c Command) Output(ctx context.Context) ([]byte, error) {
	path, err := c.Find()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, c.Args...)
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	output := &cappedBuffer{limit: c.MaxOutput}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailed, err)
	}
	if output.overflow {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrTooLarge, c.MaxOutput)
	}
	return output.data.Bytes(), nil
}

// cappedBuffer keeps at most limit bytes and remembers it dropped some.
// It must not embed bytes.Buffer: its ReadFrom would let io.Copy bypass
// the cap.
type cappedBuffer struct {
	data     bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.data.Len()+len(p) > b.limit {
		b.overflow = true
		return len(p), nil
	}
	return b.data.Write(p)
}
