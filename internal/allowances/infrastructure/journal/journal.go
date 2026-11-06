// Package journal logs one line per collected subscription (AGENTS.md,
// "Logging"); never keys, headers, bodies or file contents.
package journal

import (
	"log/slog"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// Log writes collection records through slog.
type Log struct {
	logger *slog.Logger
}

// New logs through logger.
func New(logger *slog.Logger) Log { return Log{logger: logger} }

// Record logs one collected subscription.
func (l Log) Record(record domain.CollectionRecord) {
	reading := record.Reading
	l.logger.Info("subscription collected",
		slog.String("subscription", reading.Subscription().String()),
		slog.String("source", reading.Source().String()),
		slog.String("fidelity", string(reading.Fidelity())),
		slog.String("outcome", string(reading.Outcome())),
		slog.String("error_class", string(reading.ErrorClass())),
		slog.Int64("latency_ms", record.Latency.Milliseconds()),
		slog.String("freshness", string(record.Freshness)),
		slog.Bool("asked", record.Asked),
	)
}
