package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	offeringsapp "codeberg.org/simplicer/ataecina/internal/offerings/application"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/offeringsfile"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/publishedoffers"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// updateOffers fetches the coding plan offers the maintainer publishes in
// the public repository (ADR 0017).
func updateOffers(ctx context.Context, paths xdg.Paths, env environment, client httpclient.Client) {
	endpoint, err := client.For(providerOrigin(env, publishedoffers.Origin))
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		return
	}
	current, err := offeringsapp.NewRefreshOffers(publishedoffers.New(endpoint), offeringsfile.NewOffersStore(paths.StateDir()), env.clock).Refresh(ctx)
	if err != nil {
		fmt.Fprintf(env.stdout, "offers: not updated (%v)\n", err)
		return
	}
	fmt.Fprintf(env.stdout, "offers: %d current\n", current)
}

// offerDocument is one offer of `ataecina offers --json`.
type offerDocument struct {
	ID        string  `json:"id"`
	Provider  string  `json:"provider"`
	Plan      string  `json:"plan"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Period    string  `json:"period"`
	URL       string  `json:"url"`
	Sponsored bool    `json:"sponsored"`
	Ends      *string `json:"ends"`
}

// offers lists the current offers kept by `ataecina update`; it never
// connects (ADR 0005).
func offers(args []string, env environment) int {
	flags := newFlags("offers", env)
	asJSON := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: offers: %v\n", err)
		return exitFailure
	}
	current, refreshedAt := offeringsapp.NewReportOffers(offeringsfile.NewOffersStore(paths.StateDir()), env.clock).Report()
	documents := make([]offerDocument, 0, len(current))
	for _, offer := range current {
		document := offerDocument{
			ID: offer.ID, Provider: offer.Provider, Plan: offer.Plan, Title: offer.Title, Price: offer.Price,
			Currency: offer.Currency, Period: offer.Period, URL: offer.URL, Sponsored: offer.Sponsored,
		}
		if !offer.Ends.IsZero() {
			ends := offer.Ends.Format(time.DateOnly)
			document.Ends = &ends
		}
		documents = append(documents, document)
	}
	if *asJSON {
		return writeJSON(env, map[string]any{"data": documents, "meta": map[string]any{"refreshed_at": instantOrNil(refreshedAt)}})
	}
	if len(documents) == 0 {
		fmt.Fprintln(env.stdout, "No current offers (run `ataecina update` to fetch them).")
		return exitOK
	}
	table := tabwriter.NewWriter(env.stdout, 0, 4, 2, ' ', 0)
	for _, offer := range documents {
		sponsored := ""
		if offer.Sponsored {
			sponsored = "sponsored"
		}
		fmt.Fprintf(table, "%s · %s\t%s\t%g %s / %s\t%s\t%s\n", offer.Provider, offer.Plan, offer.Title, offer.Price, offer.Currency, offer.Period, sponsored, offer.URL)
	}
	table.Flush()
	return exitOK
}
