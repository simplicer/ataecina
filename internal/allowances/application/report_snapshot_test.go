package application_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func TestReportShowsTheSnapshotAsItStandsNow(t *testing.T) {
	taken := now.Add(-4 * time.Hour)
	snapshot, _ := domain.NewSnapshot(taken, policy.Interval, []domain.Reading{meterReading(t, "claude", 80, taken)})
	report, err := application.NewReportSnapshot(&snapshotShelf{previous: snapshot}, fixedClock{now}).Report()
	if err != nil {
		t.Fatal(err)
	}
	status := report.Statuses()[0]
	if status.Freshness() != domain.FreshnessStale {
		t.Fatalf("freshness = %s", status.Freshness())
	}
	meter := status.Reading().Meters()[0]
	if meter.Used() != 0 || !meter.Estimated() {
		t.Fatalf("a window past its reset is shown reborn: %+v", meter)
	}
}

func TestReportBeforeTheFirstCollectionIsEmpty(t *testing.T) {
	report, err := application.NewReportSnapshot(&snapshotShelf{readErr: domain.ErrNoSnapshot}, fixedClock{now}).Report()
	if err != nil || len(report.Statuses()) != 0 {
		t.Fatalf("Report = %v, %d statuses", err, len(report.Statuses()))
	}
	if _, taken := report.TakenAt(); taken {
		t.Fatal("no snapshot was taken")
	}
}

func TestReportSurfacesAnUnreadableSnapshot(t *testing.T) {
	broken := errors.New("permissions too open")
	_, err := application.NewReportSnapshot(&snapshotShelf{readErr: broken}, fixedClock{now}).Report()
	if !errors.Is(err, broken) {
		t.Fatalf("Report = %v", err)
	}
}
