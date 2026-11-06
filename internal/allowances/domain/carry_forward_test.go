package domain_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

func fiveHour(used float64, resetsAt time.Time) domain.Meter {
	return domain.NewMeter("", domain.FiveHourWindow(), domain.UsedPercent(used), resetsAt)
}

func week(used float64, resetsAt time.Time) domain.Meter {
	return domain.NewMeter("", domain.WeeklyWindow(), domain.UsedPercent(used), resetsAt)
}

func TestUsageNeverDecreasesWithinAWindow(t *testing.T) {
	reset := noon.Add(2 * time.Hour)
	known := []domain.Meter{fiveHour(40, reset)}

	merged, changed := domain.CarryForward(known, []domain.Meter{fiveHour(23, reset.Add(20*time.Second))}, noon)
	if changed || merged[0].Used() != 40 {
		t.Fatalf("an idle session replaying 23%% must not hide 40%%: %v %v", merged[0].Used(), changed)
	}
	merged, changed = domain.CarryForward(known, []domain.Meter{fiveHour(55, reset)}, noon)
	if !changed || merged[0].Used() != 55 {
		t.Fatalf("higher usage in the same window wins: %v", merged[0].Used())
	}
	merged, changed = domain.CarryForward(known, []domain.Meter{fiveHour(40, reset)}, noon)
	if changed || merged[0].Used() != 40 {
		t.Fatal("the same figures bring nothing new")
	}
}

func TestALaterResetIsANewWindowAndAnEarlierOneIsAReplay(t *testing.T) {
	reset := noon.Add(2 * time.Hour)
	known := []domain.Meter{fiveHour(90, reset)}

	merged, changed := domain.CarryForward(known, []domain.Meter{fiveHour(3, reset.Add(5*time.Hour))}, noon)
	if !changed || merged[0].Used() != 3 {
		t.Fatalf("a newer window wins even with less usage: %v", merged[0].Used())
	}
	merged, changed = domain.CarryForward(known, []domain.Meter{fiveHour(95, reset.Add(-5*time.Hour))}, noon)
	if changed || merged[0].Used() != 90 {
		t.Fatalf("an older window is a replay: %v", merged[0].Used())
	}
}

func TestUnknownResetsCompareByValue(t *testing.T) {
	known := []domain.Meter{fiveHour(10, time.Time{})}
	if _, changed := domain.CarryForward(known, []domain.Meter{fiveHour(10, time.Time{})}, noon); changed {
		t.Fatal("identical figures without reset bring nothing new")
	}
	if merged, changed := domain.CarryForward(known, []domain.Meter{fiveHour(12, time.Time{})}, noon); !changed || merged[0].Used() != 12 {
		t.Fatal("different figures without reset are taken")
	}
}

func TestKnownWindowsAreKeptWhileOpen(t *testing.T) {
	known := []domain.Meter{fiveHour(10, noon.Add(time.Hour)), week(41, noon.Add(48*time.Hour))}
	merged, changed := domain.CarryForward(known, []domain.Meter{fiveHour(20, noon.Add(time.Hour))}, noon)
	if !changed || len(merged) != 2 || merged[1].Window() != domain.WeeklyWindow() {
		t.Fatalf("the week must be kept while open: %+v", merged)
	}
	merged, _ = domain.CarryForward(known, []domain.Meter{fiveHour(20, noon.Add(time.Hour))}, noon.Add(49*time.Hour))
	if len(merged) != 1 {
		t.Fatal("a closed window the report lacks is dropped")
	}
	merged, changed = domain.CarryForward(nil, []domain.Meter{week(5, noon.Add(time.Hour))}, noon)
	if !changed || len(merged) != 1 {
		t.Fatal("a first report is new")
	}
}
