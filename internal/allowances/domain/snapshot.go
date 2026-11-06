package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidSnapshot reports a snapshot that breaks the model's rules.
var ErrInvalidSnapshot = errors.New("invalid snapshot")

// Snapshot is the latest reading of every configured subscription, taken
// by the collector at one instant with one collection interval.
type Snapshot struct {
	takenAt  time.Time
	interval time.Duration
	readings []Reading
}

// NewSnapshot validates a snapshot: one reading per subscription.
func NewSnapshot(takenAt time.Time, interval time.Duration, readings []Reading) (Snapshot, error) {
	if takenAt.IsZero() || interval <= 0 {
		return Snapshot{}, fmt.Errorf("%w: needs the time it was taken and a positive interval", ErrInvalidSnapshot)
	}
	seen := make(map[SubscriptionID]bool, len(readings))
	for _, reading := range readings {
		if seen[reading.facts.Subscription] {
			return Snapshot{}, fmt.Errorf("%w: subscription %q twice", ErrInvalidSnapshot, reading.facts.Subscription)
		}
		seen[reading.facts.Subscription] = true
	}
	return Snapshot{takenAt: takenAt.UTC(), interval: interval, readings: append([]Reading(nil), readings...)}, nil
}

// TakenAt is when the collector took the snapshot; absent for an empty one.
func (s Snapshot) TakenAt() (time.Time, bool) { return s.takenAt, !s.takenAt.IsZero() }

// Interval is the collection interval the freshness rule uses.
func (s Snapshot) Interval() time.Duration { return s.interval }

// Readings are the readings, in configuration order.
func (s Snapshot) Readings() []Reading { return append([]Reading(nil), s.readings...) }

// ReadingOf finds the reading of one subscription.
func (s Snapshot) ReadingOf(id SubscriptionID) (Reading, bool) {
	for _, reading := range s.readings {
		if reading.facts.Subscription == id {
			return reading, true
		}
	}
	return Reading{}, false
}

// StatusAt applies the freshness and rebirth rules at now (SPECS §4.2).
func (s Snapshot) StatusAt(now time.Time) StatusReport {
	statuses := make([]SubscriptionStatus, len(s.readings))
	for i, reading := range s.readings {
		statuses[i] = SubscriptionStatus{reading: reading.At(now), freshness: reading.Freshness(now, s.interval)}
	}
	return StatusReport{takenAt: s.takenAt, statuses: statuses}
}

// SubscriptionStatus is one subscription as a display shows it.
type SubscriptionStatus struct {
	reading   Reading
	freshness Freshness
}

// Reading is the reading with reborn windows estimated.
func (s SubscriptionStatus) Reading() Reading { return s.reading }

// Freshness is how old the reading is.
func (s SubscriptionStatus) Freshness() Freshness { return s.freshness }

// StatusReport is what every display shows.
type StatusReport struct {
	takenAt  time.Time
	statuses []SubscriptionStatus
}

// TakenAt is when the snapshot behind the report was taken, if any.
func (r StatusReport) TakenAt() (time.Time, bool) { return r.takenAt, !r.takenAt.IsZero() }

// Statuses are the subscriptions, in configuration order.
func (r StatusReport) Statuses() []SubscriptionStatus {
	return append([]SubscriptionStatus(nil), r.statuses...)
}

// WorstUsed is the highest percentage among all meters shown.
func (r StatusReport) WorstUsed() (UsedPercent, bool) {
	var worst UsedPercent
	found := false
	for _, status := range r.statuses {
		if meter, ok := status.reading.WorstMeter(); ok && (!found || meter.used > worst) {
			worst, found = meter.used, true
		}
	}
	return worst, found
}
