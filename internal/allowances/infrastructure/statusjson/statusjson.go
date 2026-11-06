// Package statusjson writes the public `ataecina status --json` contract
// (SPECS §8.2, ADR 0010) and RFC 9457 problem objects. Changes here must
// be additive: removing or renaming a field is a breaking change.
package statusjson

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/readingjson"
)

// Meta describes the program that produced the document.
type Meta struct {
	Version     string
	GeneratedAt time.Time
}

type document struct {
	Data data `json:"data"`
	Meta meta `json:"meta"`
}

type data struct {
	Subscriptions []subscription `json:"subscriptions"`
}

type subscription struct {
	ID           string                    `json:"id"`
	Provider     string                    `json:"provider"`
	Plan         *string                   `json:"plan"`
	Outcome      string                    `json:"outcome"`
	Fidelity     string                    `json:"fidelity"`
	Freshness    string                    `json:"freshness"`
	Source       string                    `json:"source"`
	ObservedAt   *readingjson.Instant      `json:"observed_at"`
	Meters       []readingjson.Meter       `json:"meters"`
	Balance      *readingjson.Balance      `json:"balance"`
	Credits      *readingjson.Credits      `json:"credits"`
	BankedResets *readingjson.BankedResets `json:"banked_resets"`
	ErrorClass   *string                   `json:"error_class"`
	Hint         *string                   `json:"hint"`
}

type meta struct {
	Version          string               `json:"version"`
	GeneratedAt      readingjson.Instant  `json:"generated_at"`
	SnapshotTakenAt  *readingjson.Instant `json:"snapshot_taken_at"`
	WorstUsedPercent *float64             `json:"worst_used_percent"`
}

// Write renders the status report as the public JSON document.
func Write(w io.Writer, report domain.StatusReport, about Meta) error {
	doc := document{Data: data{Subscriptions: make([]subscription, 0)}}
	for _, status := range report.Statuses() {
		reading := readingjson.EncodeReading(status.Reading())
		doc.Data.Subscriptions = append(doc.Data.Subscriptions, subscription{
			ID:           reading.Subscription,
			Provider:     reading.Provider,
			Plan:         reading.Plan,
			Outcome:      reading.Outcome,
			Fidelity:     reading.Fidelity,
			Freshness:    string(status.Freshness()),
			Source:       status.Reading().Source().Label(),
			ObservedAt:   reading.ObservedAt,
			Meters:       reading.Meters,
			Balance:      reading.Balance,
			Credits:      reading.Credits,
			BankedResets: reading.BankedResets,
			ErrorClass:   reading.ErrorClass,
			Hint:         reading.Hint,
		})
	}
	takenAt, taken := report.TakenAt()
	doc.Meta = meta{
		Version:         about.Version,
		GeneratedAt:     readingjson.Instant{Time: about.GeneratedAt},
		SnapshotTakenAt: readingjson.InstantOf(takenAt, taken),
	}
	if worst, ok := report.WorstUsed(); ok {
		value := worst.Float64()
		doc.Meta.WorstUsedPercent = &value
	}
	return encode(w, doc)
}

// Problem is an RFC 9457 problem object, written on stderr.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

// NewProblem names a problem type under urn:ataecina:problem:.
func NewProblem(slug, title, detail string) Problem {
	return Problem{Type: "urn:ataecina:problem:" + slug, Title: title, Detail: detail}
}

// WriteProblem renders a problem object.
func WriteProblem(w io.Writer, problem Problem) error {
	return encode(w, problem)
}

func encode(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}
