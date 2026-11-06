// Package logging writes Loki-compatible JSON lines (log/slog) to stderr
// and to a size-capped log file.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"syscall"
)

// MaxFileBytes is the size at which the log file is rotated to "<file>.1".
const MaxFileBytes = 1 << 20

// Open returns a logger writing one JSON object per line to stderr and to
// path. The collector is one-shot, so the file is rotated when opened.
func Open(stderr io.Writer, path string, level slog.Level) (*slog.Logger, io.Closer, error) {
	info, err := os.Lstat(path)
	switch {
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return nil, nil, fmt.Errorf("check %s: %w", path, err)
	case err == nil && info.Size() >= MaxFileBytes:
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, nil, fmt.Errorf("rotate %s: %w", path, err)
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	handler := slog.NewJSONHandler(io.MultiWriter(stderr, file), &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Time(slog.TimeKey, attr.Value.Time().UTC())
			}
			return attr
		},
	})
	return slog.New(handler), file, nil
}

// Discard is a logger that writes nothing, for the display paths.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
