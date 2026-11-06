package domain

import "time"

// Meter is one windowed allowance: how much of a window is used and when
// it is reborn.
type Meter struct {
	bucket    BucketID
	window    Window
	used      UsedPercent
	resetsAt  time.Time
	estimated bool
}

// NewMeter records a meter as the source reported it. A zero resetsAt
// means the reset instant is unknown.
func NewMeter(bucket BucketID, window Window, used UsedPercent, resetsAt time.Time) Meter {
	return Meter{bucket: bucket, window: window, used: used, resetsAt: resetsAt.UTC()}
}

// Bucket is the limit this meter belongs to.
func (m Meter) Bucket() BucketID { return m.bucket }

// Window is the span the meter covers.
func (m Meter) Window() Window { return m.window }

// Used is the percentage spent.
func (m Meter) Used() UsedPercent { return m.used }

// ResetsAt is the rebirth instant, when known.
func (m Meter) ResetsAt() (time.Time, bool) { return m.resetsAt, !m.resetsAt.IsZero() }

// Estimated reports a meter shown as reborn by Ataecina rather than read.
func (m Meter) Estimated() bool { return m.estimated }

// At is the meter as it stands at now: once the clock passes its reset the
// window is reborn — 0 % used, estimated, next reset unknown — until a new
// reading arrives (SPECS §4.2).
func (m Meter) At(now time.Time) Meter {
	if m.resetsAt.IsZero() || now.Before(m.resetsAt) {
		return m
	}
	return Meter{bucket: m.bucket, window: m.window, estimated: true}
}

func (m Meter) sameSlot(other Meter) bool {
	return m.bucket == other.bucket && m.window == other.window
}

// sameResetTolerance absorbs rounding of reset instants between reports.
const sameResetTolerance = time.Minute

// CarryForward merges meters reported again by a source that may replay
// old figures (an idle Claude Code session re-drawing its status line)
// with the meters already known. For each window, a later reset means a
// newer window and wins; within the same window usage never decreases, so
// the higher figure wins; an earlier reset is an old replay and loses.
// Known windows the report lacks are kept while still open. changed tells
// whether the report brought anything newer.
func CarryForward(known, reported []Meter, now time.Time) (merged []Meter, changed bool) {
	merged = make([]Meter, 0, len(known)+len(reported))
	for _, fresh := range reported {
		previous, found := slotOf(known, fresh)
		if !found || fresh.supersedes(previous) {
			merged = append(merged, fresh)
			changed = true
			continue
		}
		merged = append(merged, previous)
	}
	for _, previous := range known {
		if _, reportedAgain := slotOf(reported, previous); !reportedAgain && now.Before(previous.resetsAt) {
			merged = append(merged, previous)
		}
	}
	return merged, changed
}

func (m Meter) supersedes(previous Meter) bool {
	if m.resetsAt.IsZero() || previous.resetsAt.IsZero() {
		return m.used != previous.used || m.resetsAt != previous.resetsAt
	}
	gap := m.resetsAt.Sub(previous.resetsAt)
	switch {
	case gap > sameResetTolerance:
		return true
	case gap < -sameResetTolerance:
		return false
	}
	return m.used > previous.used
}

func slotOf(meters []Meter, wanted Meter) (Meter, bool) {
	for _, meter := range meters {
		if meter.sameSlot(wanted) {
			return meter, true
		}
	}
	return Meter{}, false
}
