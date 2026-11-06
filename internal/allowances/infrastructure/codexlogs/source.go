package codexlogs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
)

// SourceID is the id of this source in config.json.
const SourceID domain.SourceID = "codex-logs"

// Source reads one subscription from the Codex session logs.
type Source struct {
	codexHome string
	stateDir  string
	now       func() time.Time
}

// New reads the logs under codexHome ($CODEX_HOME or ~/.codex) and keeps
// its offsets in stateDir.
func New(codexHome, stateDir string, now func() time.Time) Source {
	return Source{codexHome: codexHome, stateDir: stateDir, now: now}
}

// Read scans the logs written since the last collection and returns the
// latest rate limits Codex recorded.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	if _, err := os.Stat(s.codexHome); err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "no Codex directory at "+s.codexHome, err)
	}
	scan := scanner{
		roots:     []string{filepath.Join(s.codexHome, "sessions"), filepath.Join(s.codexHome, "archived_sessions")},
		statePath: filepath.Join(s.stateDir, subscription.ID().String()+".json"),
	}
	result, err := scan.scan(ctx, s.now())
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "the Codex logs could not be read before the deadline", err)
	}
	if len(result.latest) == 0 && result.malformed > 0 {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "the Codex log format changed: rate limits cannot be read", nil)
	}
	if len(result.latest) == 0 {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "no Codex rate limits in the session logs of the last 8 days; use Codex once", nil)
	}
	reading, err := codexlimits.Reading(subscription, SourceID, result.latest, nil)
	if errors.Is(err, codexlimits.ErrNoFigures) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, err.Error(), nil)
	}
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "the Codex rate limits are out of range", err)
	}
	return reading, nil
}
