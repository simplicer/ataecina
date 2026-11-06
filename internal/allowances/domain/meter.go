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
