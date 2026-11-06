package opencodezen

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

const (
	statsTimeout   = 15 * time.Second
	maxOutputBytes = 1 << 20
)

// statsArguments is the fixed argv: lifetime statistics (monotonic, so no
// day boundaries matter), per model, every row, as JSON, with a private
// server so no background service is left running.
var statsArguments = []string{"stats", "--all", "--models", "--limit", "1000", "--json", "--standalone"}

// Stats reads the money spent on OpenCode Zen to date from `opencode stats`.
type Stats struct {
	findOpencode func() (string, error)
}

// NewStats finds the opencode binary with findOpencode.
func NewStats(findOpencode func() (string, error)) Stats {
	return Stats{findOpencode: findOpencode}
}

// SpentToDate runs `opencode stats` once with a fixed argv and no shell.
func (s Stats) SpentToDate(ctx context.Context) (domain.Money, error) {
	path, err := s.findOpencode()
	if err != nil {
		return domain.Money{}, domain.Failure(domain.ErrorUnreadable, "opencode is not on PATH", err)
	}
	ctx, cancel := context.WithTimeout(ctx, statsTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, statsArguments...)
	output := &cappedBuffer{limit: maxOutputBytes}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		return domain.Money{}, domain.Failure(domain.ErrorUnavailable, "opencode stats did not answer", err)
	}
	if output.overflow {
		return domain.Money{}, domain.Failure(domain.ErrorContractChanged, "opencode stats answered more than 1 MiB", nil)
	}
	spent, err := DecodeSpend(output.Bytes())
	if err != nil {
		return domain.Money{}, domain.Failure(domain.ErrorContractChanged, "unexpected opencode stats output", err)
	}
	usd, _ := domain.NewCurrency("USD")
	return domain.NewMoney(spent, usd), nil
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

// Bytes is what was kept.
func (b *cappedBuffer) Bytes() []byte { return b.data.Bytes() }
