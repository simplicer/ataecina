package codexlogs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlimits"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	stateVersion = 1
	// recentFiles bounds the scan to logs touched within a weekly window
	// plus a day: older files cannot hold a window that is still open.
	recentFiles = 8 * 24 * time.Hour
	// maxLineBytes skips longer lines unread; token_count events are small.
	maxLineBytes  = 64 << 10
	maxStateBytes = 1 << 20
)

// scanState is what the reader remembers between collections: how far it
// read each file, and the latest limits per limit id. It holds no log
// content beyond those figures.
type scanState struct {
	Version int                           `json:"version"`
	Files   map[string]fileMark           `json:"files"`
	Latest  map[string]codexlimits.Limits `json:"latest"`
}

type fileMark struct {
	Inode  uint64 `json:"inode"`
	Offset int64  `json:"offset"`
}

// scanner reads the Codex session logs from the last offsets it processed.
type scanner struct {
	roots     []string
	statePath string
}

type scanResult struct {
	latest    map[string]codexlimits.Limits
	malformed int
}

func (s scanner) scan(ctx context.Context, now time.Time) (scanResult, error) {
	state := s.loadState()
	next := scanState{Version: stateVersion, Files: map[string]fileMark{}, Latest: state.Latest}
	result := scanResult{latest: state.Latest}
	for _, root := range s.roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil // a missing root or an unreadable entry is skipped
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !entry.Type().IsRegular() || !isRollout(entry.Name()) {
				return nil
			}
			info, err := entry.Info()
			if err != nil || now.Sub(info.ModTime()) > recentFiles {
				return nil
			}
			mark, malformed, err := advance(path, info, state.Files[path], result.latest)
			if err != nil {
				return nil
			}
			result.malformed += malformed
			next.Files[path] = mark
			return nil
		})
		if err != nil {
			return scanResult{}, err
		}
	}
	next.Latest = result.latest
	s.saveState(next)
	return result, nil
}

func isRollout(name string) bool {
	return strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl")
}

func inodeOf(info fs.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino
	}
	return 0
}

// advance reads what was appended to one log since its mark; a replaced or
// truncated file is read again from the start.
func advance(path string, info fs.FileInfo, mark fileMark, latest map[string]codexlimits.Limits) (fileMark, int, error) {
	inode := inodeOf(info)
	if mark.Inode != inode || mark.Offset > info.Size() {
		mark = fileMark{Inode: inode}
	}
	if mark.Offset == info.Size() {
		return mark, 0, nil
	}
	offset, malformed, err := readFrom(path, mark.Offset, latest)
	if err != nil {
		return fileMark{}, 0, err
	}
	mark.Offset = offset
	return mark, malformed, nil
}

// readFrom reads complete lines from offset, keeps the newest limits per
// limit id in latest, and returns the offset after the last complete line.
// A line still being written (no newline yet) is left for the next scan.
func readFrom(path string, offset int64, latest map[string]codexlimits.Limits) (int64, int, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return offset, 0, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return offset, 0, err
	}
	reader := bufio.NewReaderSize(file, maxLineBytes)
	malformed := 0
	lineStart, position := offset, offset
	tooLong := false
	for {
		chunk, err := reader.ReadSlice('\n')
		position += int64(len(chunk))
		if errors.Is(err, bufio.ErrBufferFull) {
			tooLong = true
			continue
		}
		if err != nil {
			// End of file inside a line: resume at its start next time.
			return lineStart, malformed, nil
		}
		if !tooLong {
			limits, ok, err := DecodeLine(chunk)
			switch {
			case err != nil:
				malformed++
			case ok && !limits.ObservedAt.Before(latest[limits.LimitID].ObservedAt):
				latest[limits.LimitID] = limits
			}
		}
		tooLong = false
		lineStart = position
	}
}

func (s scanner) loadState() scanState {
	empty := scanState{Version: stateVersion, Files: map[string]fileMark{}, Latest: map[string]codexlimits.Limits{}}
	data, err := privatefs.ReadFile(s.statePath, maxStateBytes)
	if err != nil {
		return empty
	}
	var state scanState
	if json.Unmarshal(data, &state) != nil || state.Version != stateVersion || state.Files == nil || state.Latest == nil {
		return empty
	}
	return state
}

// saveState is best effort: without it the next scan reads more, nothing worse.
func (s scanner) saveState(state scanState) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	if privatefs.EnsureDir(filepath.Dir(s.statePath)) == nil {
		_ = privatefs.WriteFile(s.statePath, data)
	}
}
