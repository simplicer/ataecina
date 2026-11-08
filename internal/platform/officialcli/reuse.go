package officialcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

// Reuse keeps what an adapter extracted from an official CLI's answer and
// gives it back, without starting the CLI again, while it is younger than
// MaxAge and none of the watched files changed since it was kept. Some
// official CLIs start a whole server to answer (OpenCode: up to 600 MB for
// a few seconds), so a collection every few minutes must not start them for
// nothing. Only the extracted figures are kept, never the CLI's own output,
// and only after they were decoded: a changed contract is asked again.
//
// A nil *Reuse keeps nothing, so adapters may call it unconditionally.
type Reuse struct {
	// File keeps the extracted figures (in the state directory, 0600).
	File string
	// Watch lists files whose change calls for a new answer; their times
	// are read when the figures are kept, after the CLI ran, so what the
	// CLI itself wrote does not count as a change.
	Watch  []string
	MaxAge time.Duration
	// Now is the clock (time.Now when nil).
	Now func() time.Time
}

// keptLimit bounds the kept file: it holds a few figures only.
const keptLimit = 64 << 10

type kept struct {
	TakenAt time.Time        `json:"taken_at"`
	Watched map[string]int64 `json:"watched"`
	Value   json.RawMessage  `json:"value"`
}

// Recall fills into with the kept figures when they still hold.
func (r *Reuse) Recall(into any) bool {
	if r == nil {
		return false
	}
	data, err := privatefs.ReadFile(r.File, keptLimit)
	if err != nil {
		return false
	}
	var last kept
	if err := json.Unmarshal(data, &last); err != nil {
		return false
	}
	age := r.now().Sub(last.TakenAt)
	if age < 0 || age >= r.MaxAge || !sameTimes(r.watched(), last.Watched) {
		return false
	}
	return json.Unmarshal(last.Value, into) == nil
}

// Keep stores the extracted figures; a failure only means the next
// collection asks the CLI again.
func (r *Reuse) Keep(value any) {
	if r == nil {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	data, err := json.Marshal(kept{TakenAt: r.now().UTC(), Watched: r.watched(), Value: encoded})
	if err != nil || privatefs.EnsureDir(filepath.Dir(r.File)) != nil {
		return
	}
	_ = privatefs.WriteFile(r.File, data)
}

func (r *Reuse) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// watched is each watched file's modification time (0 when missing).
func (r *Reuse) watched() map[string]int64 {
	times := make(map[string]int64, len(r.Watch))
	for _, path := range r.Watch {
		times[path] = 0
		if info, err := os.Stat(path); err == nil {
			times[path] = info.ModTime().UnixNano()
		}
	}
	return times
}

func sameTimes(current, kept map[string]int64) bool {
	if len(current) != len(kept) {
		return false
	}
	for path, changed := range current {
		if when, ok := kept[path]; !ok || when != changed {
			return false
		}
	}
	return true
}
