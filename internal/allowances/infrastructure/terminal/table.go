// Package terminal draws the status report as a table for people. Its
// layout is not a contract and may change freely (use --json for scripts).
package terminal

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

const (
	warningPercent  = 90
	criticalPercent = 95
)

type colour string

const (
	plain    colour = ""
	red      colour = "31"
	yellow   colour = "33"
	dim      colour = "2"
	resetSGR        = "\x1b[0m"
)

type cell struct {
	text   string
	colour colour
}

// Render writes the table. Colours are used only when colourful is true
// (stdout is a terminal and NO_COLOR is unset).
func Render(w io.Writer, report domain.StatusReport, now time.Time, colourful bool) error {
	takenAt, taken := report.TakenAt()
	if !taken {
		_, err := fmt.Fprintln(w, "No snapshot yet. Run `ataecina collect`, or enable ataecina-collect.timer.")
		return err
	}
	rows := [][]cell{{{text: "SUBSCRIPTION"}, {text: "PLAN"}, {text: "WORST"}, {text: "WINDOWS"}, {text: "BALANCE"}, {text: "STATE"}}}
	for _, status := range report.Statuses() {
		rows = append(rows, row(status, now))
	}
	header := fmt.Sprintf("Ataecina · updated %s", age(now.Sub(takenAt)))
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	return writeAligned(w, rows, colourful)
}

func row(status domain.SubscriptionStatus, now time.Time) []cell {
	reading := status.Reading()
	plan := reading.Plan().String()
	if plan == "" {
		plan = "—"
	}
	worst := cell{text: "—"}
	if meter, ok := reading.WorstMeter(); ok {
		worst = cell{text: percent(meter.Used()), colour: severity(meter.Used())}
	}
	return []cell{
		{text: reading.Subscription().String()},
		{text: plan},
		worst,
		{text: windows(reading, now)},
		{text: money(reading)},
		state(status, now),
	}
}

func windows(reading domain.Reading, now time.Time) string {
	meters := reading.Meters()
	if len(meters) == 0 {
		return "—"
	}
	first := meters[0].Bucket()
	parts := make([]string, 0, len(meters))
	for _, meter := range meters {
		label := windowLabel(meter.Window())
		if meter.Bucket() != first {
			label = meter.Bucket().String() + " " + label
		}
		text := label + " " + percent(meter.Used())
		if resetsAt, ok := meter.ResetsAt(); ok {
			text += " ↻ " + countdown(resetsAt.Sub(now))
		}
		if meter.Estimated() {
			text += " (reborn)"
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " · ")
}

func money(reading domain.Reading) string {
	var parts []string
	if balance, ok := reading.Balance(); ok {
		approximate := ""
		if reading.Fidelity() != domain.FidelityReported {
			approximate = "≈ "
		}
		if remaining, ok := balance.Remaining(); ok {
			parts = append(parts, approximate+amount(remaining)+" left")
		}
		if spent, ok := balance.Spent(); ok {
			parts = append(parts, amount(spent)+" spent")
		}
	}
	if credits, ok := reading.Credits(); ok {
		if credits.Unlimited() {
			parts = append(parts, "credits ∞")
		} else {
			parts = append(parts, "credits "+credits.Available().String())
		}
	}
	if banked, ok := reading.BankedResets(); ok && banked.Available() > 0 {
		parts = append(parts, fmt.Sprintf("%d banked resets", banked.Available()))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}

func state(status domain.SubscriptionStatus, now time.Time) cell {
	reading := status.Reading()
	var parts []string
	tone := plain
	if reading.Outcome() != domain.OutcomeOK {
		parts = append(parts, words(string(reading.Outcome())))
		tone = red
		if reading.Outcome() == domain.OutcomePartial || reading.Outcome() == domain.OutcomeUnsupported {
			tone = dim
		}
	}
	if observedAt, ok := reading.ObservedAt(); ok {
		parts = append(parts, words(string(status.Freshness()))+" "+age(now.Sub(observedAt)))
		if status.Freshness() == domain.FreshnessStale && tone == plain {
			tone = dim
		}
	}
	if reading.Fidelity() != domain.FidelityReported {
		parts = append(parts, words(string(reading.Fidelity())))
	}
	if class := reading.ErrorClass(); class != "" && reading.Outcome().HasFigures() {
		parts = append(parts, "last try "+words(string(class)))
	}
	text := strings.Join(parts, " · ")
	if hint := reading.Hint(); hint != "" {
		text += ": " + hint
	}
	return cell{text: text, colour: tone}
}

func writeAligned(w io.Writer, rows [][]cell, colourful bool) error {
	widths := make([]int, len(rows[0]))
	for _, cells := range rows {
		for i, c := range cells {
			widths[i] = max(widths[i], utf8.RuneCountInString(c.text))
		}
	}
	for _, cells := range rows {
		var line strings.Builder
		for i, c := range cells {
			text := c.text
			if colourful && c.colour != plain {
				text = "\x1b[" + string(c.colour) + "m" + text + resetSGR
			}
			line.WriteString(text)
			if i < len(cells)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c.text)+2))
			}
		}
		if _, err := fmt.Fprintln(w, strings.TrimRight(line.String(), " ")); err != nil {
			return err
		}
	}
	return nil
}

func severity(used domain.UsedPercent) colour {
	switch {
	case used.Float64() >= criticalPercent:
		return red
	case used.Float64() >= warningPercent:
		return yellow
	}
	return plain
}

func percent(used domain.UsedPercent) string {
	return strconv.FormatFloat(math.Round(used.Float64()*10)/10, 'f', -1, 64) + "%"
}

func amount(m domain.Money) string {
	value := fmt.Sprintf("%.2f", m.Amount().Float64())
	switch m.Currency().String() {
	case "USD":
		return "$" + value
	case "EUR":
		return "€" + value
	case "GBP":
		return "£" + value
	}
	return value + " " + m.Currency().String()
}

func windowLabel(window domain.Window) string {
	switch window.Kind() {
	case domain.WindowFiveHours:
		return "5h"
	case domain.WindowDaily:
		return "day"
	case domain.WindowWeekly:
		return "week"
	case domain.WindowMonthly:
		return "month"
	case domain.WindowBillingCycle:
		return "cycle"
	}
	minutes := window.Minutes()
	switch {
	case minutes%1440 == 0:
		return fmt.Sprintf("%dd", minutes/1440)
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}

func countdown(left time.Duration) string {
	if left < time.Minute {
		return "<1m"
	}
	left = left.Round(time.Minute)
	days, hours, minutes := int(left/(24*time.Hour)), int(left%(24*time.Hour)/time.Hour), int(left%time.Hour/time.Minute)
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%02dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func age(elapsed time.Duration) string {
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed/time.Minute))
	case elapsed < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed/time.Hour))
	}
	return fmt.Sprintf("%dd ago", int(elapsed/(24*time.Hour)))
}

func words(enum string) string {
	return strings.ToLower(strings.ReplaceAll(enum, "_", " "))
}
