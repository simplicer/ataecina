package application_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

type signalShelf struct {
	signal   *domain.Reading
	readErr  error
	keepErr  error
	kept     int
	lastKept domain.Reading
}

func (s *signalShelf) LatestSignal(domain.SubscriptionID) (domain.Reading, error) {
	if s.readErr != nil {
		return domain.Reading{}, s.readErr
	}
	if s.signal == nil {
		return domain.Reading{}, domain.ErrNoSignal
	}
	return *s.signal, nil
}

func (s *signalShelf) KeepSignal(reading domain.Reading) error {
	s.kept++
	s.lastKept = reading
	if s.keepErr != nil {
		return s.keepErr
	}
	s.signal = &reading
	return nil
}

var claude = domain.NewSubscription("claude", "anthropic", "claude-statusline", domain.FidelityReported, "")

func meter(window domain.Window, used float64, resetsAt time.Time) domain.Meter {
	return domain.NewMeter("", window, domain.UsedPercent(used), resetsAt)
}

func record(shelf *signalShelf, at time.Time, meters ...domain.Meter) (domain.Reading, error) {
	return application.NewRecordClaudeLimits(shelf, fixedClock{at}).Record(claude, meters)
}

func TestTheFirstLimitsAreKept(t *testing.T) {
	shelf := &signalShelf{}
	reading, err := record(shelf, now,
		meter(domain.FiveHourWindow(), 23, now.Add(2*time.Hour)),
		meter(domain.WeeklyWindow(), 41, now.Add(100*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if shelf.kept != 1 || len(reading.Meters()) != 2 || reading.Source() != "claude-statusline" || reading.Fidelity() != domain.FidelityReported {
		t.Fatalf("reading = %+v, kept %d", reading, shelf.kept)
	}
	if observed, _ := reading.ObservedAt(); !observed.Equal(now) {
		t.Fatalf("observed at %v", observed)
	}
}

func TestAReplayOfOlderFiguresIsNotWritten(t *testing.T) {
	shelf := &signalShelf{}
	reset := now.Add(2 * time.Hour)
	if _, err := record(shelf, now, meter(domain.FiveHourWindow(), 40, reset)); err != nil {
		t.Fatal(err)
	}
	reading, err := record(shelf, now.Add(time.Minute), meter(domain.FiveHourWindow(), 23, reset))
	if err != nil || shelf.kept != 1 || reading.Meters()[0].Used() != 40 {
		t.Fatalf("replay = %v, kept %d", err, shelf.kept)
	}
	reading, err = record(shelf, now.Add(2*time.Minute), meter(domain.FiveHourWindow(), 45, reset))
	if err != nil || shelf.kept != 2 || reading.Meters()[0].Used() != 45 {
		t.Fatalf("newer = %v, kept %d", err, shelf.kept)
	}
	if observed, _ := reading.ObservedAt(); !observed.Equal(now.Add(2 * time.Minute)) {
		t.Fatal("observed_at moves only when figures change")
	}
}

func TestNoLimitsInTheInputShowWhatIsKnown(t *testing.T) {
	if _, err := record(&signalShelf{}, now); !errors.Is(err, domain.ErrNoSignal) {
		t.Fatalf("nothing known = %v", err)
	}
	known := meterReading(t, "claude", 12, now)
	reading, err := record(&signalShelf{signal: &known}, now)
	if err != nil || reading.Meters()[0].Used() != 12 {
		t.Fatalf("known = %v", err)
	}
}

func TestAnUnreadableSignalIsReplaced(t *testing.T) {
	shelf := &signalShelf{readErr: errors.New("corrupt")}
	if _, err := record(shelf, now, meter(domain.FiveHourWindow(), 5, now.Add(time.Hour))); err != nil || shelf.kept != 1 {
		t.Fatalf("Record = %v, kept %d", err, shelf.kept)
	}
}

func TestFailuresAreReturnedForTheCallerToIgnore(t *testing.T) {
	shelf := &signalShelf{keepErr: errors.New("read-only")}
	reading, err := record(shelf, now, meter(domain.FiveHourWindow(), 5, now.Add(time.Hour)))
	if err == nil || len(reading.Meters()) != 1 {
		t.Fatalf("Record = %v", err)
	}
	twice := meter(domain.FiveHourWindow(), 5, now.Add(time.Hour))
	if _, err := record(&signalShelf{}, now, twice, twice); !errors.Is(err, domain.ErrInvalidReading) {
		t.Fatalf("duplicated windows = %v", err)
	}
}
