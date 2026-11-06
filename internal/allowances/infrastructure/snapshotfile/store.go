// Package snapshotfile keeps the snapshot in
// $XDG_RUNTIME_DIR/ataecina/snapshot.json (0600, tmpfs).
package snapshotfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/readingjson"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	formatVersion = 1
	maxBytes      = 4 << 20
	fileName      = "snapshot.json"
)

// ErrCorrupt reports a snapshot file Ataecina cannot read.
var ErrCorrupt = errors.New("snapshot file is corrupt")

type document struct {
	Version         int                   `json:"version"`
	TakenAt         readingjson.Instant   `json:"taken_at"`
	IntervalSeconds int64                 `json:"interval_seconds"`
	Readings        []readingjson.Reading `json:"readings"`
}

// Store reads and writes the snapshot file in one runtime directory.
type Store struct {
	dir string
}

// NewStore keeps the snapshot in dir.
func NewStore(dir string) Store { return Store{dir: dir} }

// Path is the snapshot file.
func (s Store) Path() string { return filepath.Join(s.dir, fileName) }

// LatestSnapshot reads the snapshot, or domain.ErrNoSnapshot.
func (s Store) LatestSnapshot() (domain.Snapshot, error) {
	data, err := privatefs.ReadFile(s.Path(), maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Snapshot{}, domain.ErrNoSnapshot
	}
	if err != nil {
		return domain.Snapshot{}, err
	}
	var stored document
	if err := json.Unmarshal(data, &stored); err != nil {
		return domain.Snapshot{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if stored.Version != formatVersion {
		return domain.Snapshot{}, fmt.Errorf("%w: format version %d", ErrCorrupt, stored.Version)
	}
	readings := make([]domain.Reading, 0, len(stored.Readings))
	for _, item := range stored.Readings {
		reading, err := readingjson.DecodeReading(item)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
		}
		readings = append(readings, reading)
	}
	snapshot, err := domain.NewSnapshot(stored.TakenAt.Time, time.Duration(stored.IntervalSeconds)*time.Second, readings)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	return snapshot, nil
}

// KeepSnapshot writes the snapshot atomically in a private directory.
func (s Store) KeepSnapshot(snapshot domain.Snapshot) error {
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	takenAt, _ := snapshot.TakenAt()
	stored := document{
		Version:         formatVersion,
		TakenAt:         readingjson.Instant{Time: takenAt},
		IntervalSeconds: int64(snapshot.Interval() / time.Second),
		Readings:        make([]readingjson.Reading, 0),
	}
	for _, reading := range snapshot.Readings() {
		stored.Readings = append(stored.Readings, readingjson.EncodeReading(reading))
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	return privatefs.WriteFile(s.Path(), append(data, '\n'))
}
