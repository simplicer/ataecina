package terminal

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// NothingYet is the summary before any limit is known.
const NothingYet = "5h – · 7d –"

// Summary is one compact line for a status line: every window with its
// whole percentage, then the time to the nearest rebirth
// ("5h 23% · 7d 41% · ↻ 2h10m").
func Summary(reading domain.Reading, now time.Time) string {
	meters := reading.At(now).Meters()
	if len(meters) == 0 {
		return NothingYet
	}
	parts := make([]string, 0, len(meters)+1)
	var nearest time.Time
	for _, meter := range meters {
		label := compactLabel(meter.Window())
		if !meter.Bucket().IsMain() {
			label = meter.Bucket().String() + " " + label
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", label, meter.Used().Float64()))
		if resetsAt, ok := meter.ResetsAt(); ok && (nearest.IsZero() || resetsAt.Before(nearest)) {
			nearest = resetsAt
		}
	}
	if !nearest.IsZero() {
		parts = append(parts, "↻ "+countdown(nearest.Sub(now)))
	}
	return strings.Join(parts, " · ")
}

func compactLabel(window domain.Window) string {
	if window.Kind() == domain.WindowWeekly {
		return "7d"
	}
	return windowLabel(window)
}
