package claudestatusline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/readingjson"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	signalVersion  = 1
	maxSignalBytes = 64 << 10
)

type signalDocument struct {
	Version int                 `json:"version"`
	Reading readingjson.Reading `json:"reading"`
}

// Signals keeps one signal file per subscription in a private directory.
type Signals struct {
	dir string
}

// NewSignals keeps signals in dir ($XDG_RUNTIME_DIR/ataecina/signals).
func NewSignals(dir string) Signals { return Signals{dir: dir} }

func (s Signals) path(id domain.SubscriptionID) string {
	return filepath.Join(s.dir, id.String()+".json")
}

// LatestSignal reads a subscription's signal, or domain.ErrNoSignal.
func (s Signals) LatestSignal(id domain.SubscriptionID) (domain.Reading, error) {
	data, err := privatefs.ReadFile(s.path(id), maxSignalBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Reading{}, domain.ErrNoSignal
	}
	if err != nil {
		return domain.Reading{}, err
	}
	var signal signalDocument
	if err := json.Unmarshal(data, &signal); err != nil || signal.Version != signalVersion {
		return domain.Reading{}, fmt.Errorf("signal %s is unreadable", s.path(id))
	}
	return readingjson.DecodeReading(signal.Reading)
}

// KeepSignal writes a subscription's signal atomically.
func (s Signals) KeepSignal(reading domain.Reading) error {
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	data, err := json.Marshal(signalDocument{Version: signalVersion, Reading: readingjson.EncodeReading(reading)})
	if err != nil {
		return err
	}
	return privatefs.WriteFile(s.path(reading.Subscription()), data)
}
