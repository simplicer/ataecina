// Package alertfile keeps the alert state in
// $XDG_STATE_HOME/ataecina/alerts.json (0600), keyed by subscription,
// bucket, window and reset instant (SPECS §7).
package alertfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	formatVersion = 1
	maxBytes      = 1 << 20
)

type document struct {
	Version int             `json:"version"`
	Raised  []raisedElement `json:"raised"`
}

type raisedElement struct {
	Subscription string    `json:"subscription"`
	Bucket       string    `json:"bucket,omitempty"`
	Window       string    `json:"window"`
	ResetsAt     time.Time `json:"resets_at"`
	Threshold    int       `json:"threshold"`
	RaisedAt     time.Time `json:"raised_at"`
}

// Store reads and writes alerts.json in the state directory.
type Store struct {
	dir string
}

// NewStore keeps the alert state in dir.
func NewStore(dir string) Store { return Store{dir: dir} }

func (s Store) path() string { return filepath.Join(s.dir, "alerts.json") }

// Load reads the state; a missing file is an empty state.
func (s Store) Load() (domain.AlertState, error) {
	data, err := privatefs.ReadFile(s.path(), maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.NewAlertState(nil), nil
	}
	if err != nil {
		return domain.AlertState{}, err
	}
	var stored document
	if err := json.Unmarshal(data, &stored); err != nil || stored.Version != formatVersion {
		return domain.AlertState{}, fmt.Errorf("%s is not an alert state Ataecina can read", s.path())
	}
	raised := make([]domain.RaisedAlert, 0, len(stored.Raised))
	for _, item := range stored.Raised {
		threshold, err := domain.NewThreshold(item.Threshold)
		if err != nil {
			return domain.AlertState{}, fmt.Errorf("%s: %w", s.path(), err)
		}
		raised = append(raised, domain.RaisedAlert{
			Instance:  domain.WindowInstance{Subscription: item.Subscription, Bucket: item.Bucket, Window: item.Window, ResetsAt: item.ResetsAt.UTC()},
			Threshold: threshold,
			RaisedAt:  item.RaisedAt.UTC(),
		})
	}
	return domain.NewAlertState(raised), nil
}

// Save writes the state atomically.
func (s Store) Save(state domain.AlertState) error {
	stored := document{Version: formatVersion, Raised: []raisedElement{}}
	for _, raised := range state.Raised() {
		stored.Raised = append(stored.Raised, raisedElement{
			Subscription: raised.Instance.Subscription,
			Bucket:       raised.Instance.Bucket,
			Window:       raised.Instance.Window,
			ResetsAt:     raised.Instance.ResetsAt,
			Threshold:    int(raised.Threshold),
			RaisedAt:     raised.RaisedAt,
		})
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	return privatefs.WriteFile(s.path(), append(data, '\n'))
}
