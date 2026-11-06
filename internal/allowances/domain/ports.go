package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNoSnapshot reports that the collector has not taken a snapshot yet.
var ErrNoSnapshot = errors.New("no snapshot has been taken yet")

// Source reads one subscription's allowance. Failures are returned as
// *SourceFailure so the collector knows what to keep.
type Source interface {
	Read(ctx context.Context, subscription Subscription) (Reading, error)
}

// SourceCatalog finds the source adapter of a configured subscription.
type SourceCatalog interface {
	SourceOf(subscription Subscription) (Source, bool)
}

// SnapshotReader gives the latest snapshot, or ErrNoSnapshot.
type SnapshotReader interface {
	LatestSnapshot() (Snapshot, error)
}

// SnapshotWriter keeps a new snapshot for every display.
type SnapshotWriter interface {
	KeepSnapshot(snapshot Snapshot) error
}

// CollectionRecord is what the collector reports for one subscription.
type CollectionRecord struct {
	Reading   Reading
	Freshness Freshness
	Latency   time.Duration
	Asked     bool
}

// CollectionJournal records one line per collected subscription.
type CollectionJournal interface {
	Record(record CollectionRecord)
}

// ErrNoSignal reports that no hook has recorded a signal yet.
var ErrNoSignal = errors.New("no signal recorded yet")

// LimitSignals keeps what a hook observed (Claude Code's status line) for
// the collector to read later.
type LimitSignals interface {
	LatestSignal(subscription SubscriptionID) (Reading, error)
	KeepSignal(reading Reading) error
}

// Clock tells the time; the domain itself never reads it.
type Clock interface {
	Now() time.Time
}
