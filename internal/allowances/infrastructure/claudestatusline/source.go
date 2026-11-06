package claudestatusline

import (
	"context"
	"errors"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// SourceID is the id of this source in config.json.
const SourceID domain.SourceID = "claude-statusline"

// Source gives the collector the signal the hook recorded.
type Source struct {
	signals Signals
}

// NewSource reads the signals in dir.
func NewSource(dir string) Source { return Source{signals: NewSignals(dir)} }

// Read returns the latest signal of the subscription.
func (s Source) Read(_ context.Context, subscription domain.Subscription) (domain.Reading, error) {
	reading, err := s.signals.LatestSignal(subscription.ID())
	if errors.Is(err, domain.ErrNoSignal) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable,
			"no Claude Code status line signal yet: add `ataecina hook claude-statusline` to the status line and send one message", err)
	}
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "the Claude signal file cannot be read", err)
	}
	return reading, nil
}
