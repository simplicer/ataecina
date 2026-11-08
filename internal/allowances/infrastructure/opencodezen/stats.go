package opencodezen

import (
	"context"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// statsArguments is the fixed argv: lifetime statistics (monotonic, so no
// day boundaries matter), per model, every row, as JSON, with a private
// server so no background service is left running.
var statsArguments = []string{"stats", "--all", "--models", "--limit", "1000", "--json", "--standalone"}

// Stats reads the money spent on OpenCode Zen to date from `opencode stats`.
type Stats struct {
	command officialcli.Command
	reuse   *officialcli.Reuse
}

// keptSpend is all that is kept of an answer (officialcli.Reuse).
type keptSpend struct {
	Spent string `json:"spent_usd"`
}

// NewStats finds the opencode binary with findOpencode; with reuse set, a
// recent spend is given again instead of starting opencode (nil: always
// run it).
func NewStats(findOpencode func() (string, error), reuse *officialcli.Reuse) Stats {
	return Stats{reuse: reuse, command: officialcli.Command{
		Find: findOpencode, Args: statsArguments, Timeout: 15 * time.Second, MaxOutput: 1 << 20,
	}}
}

// SpentToDate gives the kept spend while it holds, else runs `opencode
// stats` once with a fixed argv and no shell.
func (s Stats) SpentToDate(ctx context.Context) (domain.Money, error) {
	usd, _ := domain.NewCurrency("USD")
	var kept keptSpend
	if s.reuse.Recall(&kept) {
		if spent, err := domain.ParseAmount(kept.Spent); err == nil {
			return domain.NewMoney(spent, usd), nil
		}
	}
	output, err := s.command.Output(ctx)
	switch {
	case errors.Is(err, officialcli.ErrNotFound):
		return domain.Money{}, domain.Failure(domain.ErrorUnreadable, "opencode is not on PATH", err)
	case errors.Is(err, officialcli.ErrTooLarge):
		return domain.Money{}, domain.Failure(domain.ErrorContractChanged, "opencode stats answered more than 1 MiB", nil)
	case err != nil:
		return domain.Money{}, domain.Failure(domain.ErrorUnavailable, "opencode stats did not answer", err)
	}
	spent, err := DecodeSpend(output)
	if err != nil {
		return domain.Money{}, domain.Failure(domain.ErrorContractChanged, "unexpected opencode stats output", err)
	}
	s.reuse.Keep(keptSpend{Spent: spent.String()})
	return domain.NewMoney(spent, usd), nil
}
