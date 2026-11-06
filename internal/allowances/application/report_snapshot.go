package application

import (
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// ReportSnapshot gives every display the latest snapshot as it stands now,
// with freshness and rebirth applied. It never touches keys or the network.
type ReportSnapshot struct {
	snapshots domain.SnapshotReader
	clock     domain.Clock
}

// NewReportSnapshot prepares the report for displays.
func NewReportSnapshot(snapshots domain.SnapshotReader, clock domain.Clock) ReportSnapshot {
	return ReportSnapshot{snapshots: snapshots, clock: clock}
}

// Report reads the snapshot; before the first collection it is empty.
func (r ReportSnapshot) Report() (domain.StatusReport, error) {
	snapshot, err := r.snapshots.LatestSnapshot()
	if errors.Is(err, domain.ErrNoSnapshot) {
		return domain.Snapshot{}.StatusAt(r.clock.Now()), nil
	}
	if err != nil {
		return domain.StatusReport{}, fmt.Errorf("report snapshot: %w", err)
	}
	return snapshot.StatusAt(r.clock.Now()), nil
}
