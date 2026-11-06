package codexappserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
)

// SourceID is the id of this source in config.json.
const SourceID domain.SourceID = "codex-app-server"

// sessionTimeout bounds one app-server session (SPECS §3.2).
const sessionTimeout = 15 * time.Second

// methodNotFound is the JSON-RPC code for an unknown method.
const methodNotFound = -32601

// Source asks `codex app-server` for one subscription's limits.
type Source struct {
	findCodex func() (string, error)
	version   string
	now       func() time.Time
}

// New finds the codex binary with findCodex (FindOnPath in production)
// and introduces itself to the server as ataecina/version.
func New(findCodex func() (string, error), version string, now func() time.Time) Source {
	return Source{findCodex: findCodex, version: version, now: now}
}

// FindOnPath finds an executable `codex` in the directories of a PATH
// value, as the shell would, without running anything.
func FindOnPath(path string) func() (string, error) {
	return func() (string, error) {
		for _, dir := range filepath.SplitList(path) {
			if !filepath.IsAbs(dir) {
				continue
			}
			candidate := filepath.Join(dir, "codex")
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				return candidate, nil
			}
		}
		return "", exec.ErrNotFound
	}
}

// Read runs one app-server session and maps its answer.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	codexPath, err := s.findCodex()
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "codex is not on PATH", err)
	}
	ctx, cancel := context.WithTimeout(ctx, sessionTimeout)
	defer cancel()
	result, err := readRateLimits(ctx, codexPath, s.version)
	var answered rpcError
	switch {
	case errors.As(err, &answered) && answered.Code == methodNotFound:
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "codex app-server does not know account/rateLimits/read", err)
	case errors.As(err, &answered):
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, fmt.Sprintf("codex app-server answered error %d (is Codex logged in?)", answered.Code), err)
	case err != nil:
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "codex app-server did not answer", err)
	}
	latest, banked, err := DecodeRateLimits(result, s.now())
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected rate limits from codex app-server", err)
	}
	reading, err := codexlimits.Reading(subscription, SourceID, latest, banked)
	if errors.Is(err, codexlimits.ErrNoFigures) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, err.Error(), nil)
	}
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "the Codex rate limits are out of range", err)
	}
	return reading, nil
}

// Fallback reads the Codex logs first and asks the app server only when
// they fail or their reading is older than after (SPECS §3.2). The logs
// remain the source whenever the app server cannot answer.
type Fallback struct {
	logs   domain.Source
	server domain.Source
	after  time.Duration
	now    func() time.Time
}

// NewFallback combines the logs and the app server.
func NewFallback(logs, server domain.Source, after time.Duration, now func() time.Time) Fallback {
	return Fallback{logs: logs, server: server, after: after, now: now}
}

// Read returns the freshest reading the two sources can give.
func (f Fallback) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	fromLogs, logsErr := f.logs.Read(ctx, subscription)
	if logsErr == nil {
		if observedAt, ok := fromLogs.ObservedAt(); ok && f.now().Sub(observedAt) < f.after {
			return fromLogs, nil
		}
	}
	fromServer, serverErr := f.server.Read(ctx, subscription)
	if serverErr == nil {
		return fromServer, nil
	}
	if logsErr == nil {
		return fromLogs, nil
	}
	return domain.Reading{}, logsErr
}
