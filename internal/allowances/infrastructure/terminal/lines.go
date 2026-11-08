package terminal

import (
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// NoSnapshot is what a display shows before the first collection.
const NoSnapshot = "No snapshot yet. Run `ataecina collect`, or enable ataecina-collect.timer."

// Line is one subscription on one plain line without colours, for menus
// and tooltips: "claude · PRO · 5h 23.5% ↻ 2h10m · week 41.2% ↻ 4d04h · fresh just now".
func Line(status domain.SubscriptionStatus, now time.Time) string {
	cells := row(status, now, domain.RatePeriod{}, false)
	parts := make([]string, 0, len(cells))
	for i, cell := range cells {
		if i == worstColumn || cell.text == "—" || cell.text == "" {
			continue
		}
		parts = append(parts, cell.text)
	}
	return strings.Join(parts, " · ")
}

// Updated says how old the snapshot is ("updated 2m ago").
func Updated(takenAt, now time.Time) string {
	return "updated " + age(now.Sub(takenAt))
}

// Attention reports whether any meter has reached the warning level that
// the displays highlight (90 %, the default first threshold).
func Attention(report domain.StatusReport) bool {
	worst, ok := report.WorstUsed()
	return ok && worst.Float64() >= warningPercent
}
