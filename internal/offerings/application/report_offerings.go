package application

import (
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// ReportOfferings reads the kept catalogue for the displays; it never asks
// the source (ADR 0005).
type ReportOfferings struct {
	catalogues domain.Catalogues
	clock      domain.Clock
}

// NewReportOfferings reads the catalogue kept by the collector.
func NewReportOfferings(catalogues domain.Catalogues, clock domain.Clock) ReportOfferings {
	return ReportOfferings{catalogues: catalogues, clock: clock}
}

// Report summarises each subscription's provider the catalogue holds, and
// tells when it was last refreshed; without a catalogue it reports nothing.
func (r ReportOfferings) Report() ([]domain.Summary, time.Time) {
	catalogue, err := r.catalogues.Load()
	if err != nil {
		return nil, time.Time{}
	}
	return catalogue.Summaries(r.clock.Now()), catalogue.RefreshedAt
}
