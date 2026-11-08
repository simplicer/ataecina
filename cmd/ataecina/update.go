package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	sealedkeys "codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/keyring"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/snapshotfile"
	keysapp "codeberg.org/simplicer/ataecina/internal/keys/application"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	offeringsapp "codeberg.org/simplicer/ataecina/internal/offerings/application"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/artificialanalysis"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/modelsdev"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/offeringsfile"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// rankingKey is the sealed key of the Artificial Analysis Data API.
const rankingKey = "artificial-analysis"

// update refreshes, on the owner's request, what is not refreshed on its
// own: the model catalogue at once and the intelligence ranking (ADR
// 0016). It never touches the readings.
func update(ctx context.Context, args []string, env environment) int {
	flags := newFlags("update", env)
	fd := passphraseFlag(flags)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		return exitFailure
	}
	settings, err := configfile.Load(paths.ConfigFile(), env.sources.catalog)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		return exitFailure
	}
	client := httpclient.New(httpclient.Options{Version: version, UseEnvironmentProxy: settings.UseEnvironmentProxy, AllowPlainHTTP: plainHTTPAllowed})
	code := exitOK
	if err := updateCatalogue(ctx, settings, paths, env, client); err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		code = exitFailure
	}
	updateRanking(ctx, paths, env, client, *fd)
	return code
}

func updateCatalogue(ctx context.Context, settings configfile.Settings, paths xdg.Paths, env environment, client httpclient.Client) error {
	endpoint, err := client.For(providerOrigin(env, modelsdev.Origin))
	if err != nil {
		return err
	}
	interests := catalogueInterests(settings)
	refresh := offeringsapp.NewRefreshOfferingsNow(modelsdev.New(endpoint), offeringsfile.NewStore(paths.StateDir()), env.clock)
	if _, err := refresh.Refresh(ctx, interests); err != nil {
		return err
	}
	fmt.Fprintf(env.stdout, "model catalogue: up to date for %d subscriptions\n", len(interests))
	return nil
}

// updateRanking refreshes the intelligence index when its key is sealed;
// without it the catalogue alone is updated.
func updateRanking(ctx context.Context, paths xdg.Paths, env environment, client httpclient.Client, fd int) {
	endpoint, err := client.For(providerOrigin(env, artificialanalysis.Origin))
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		return
	}
	ring, err := openKeyring(env, fd, false)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: update: %v\n", err)
		return
	}
	keys := sealedkeys.New(keysapp.NewUnsealKey(ring.files, aesgcm.Cipher{}, ring.unlockers))
	key := func(ctx context.Context) (secret.Key, error) { return keys.KeyFor(ctx, rankingKey) }
	ranking, err := offeringsapp.NewRefreshRanking(artificialanalysis.New(endpoint, key), offeringsfile.NewRankingStore(paths.StateDir()), env.clock).Refresh(ctx)
	if err != nil {
		fmt.Fprintf(env.stdout, "intelligence index: not updated (%v); a free key from https://artificialanalysis.ai goes in with `ataecina keys set %s`\n", err, rankingKey)
		return
	}
	fmt.Fprintf(env.stdout, "intelligence index: %d models, Artificial Analysis Intelligence Index v%s\n", len(ranking.Scores), ranking.Version)
}

// modelDocument is one model of `ataecina models --json`.
type modelDocument struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Free         bool     `json:"free"`
	InputPrice   *float64 `json:"input_price"`
	OutputPrice  *float64 `json:"output_price"`
	ReleasedOn   *string  `json:"released_on"`
	Intelligence *float64 `json:"intelligence"`
	Available    bool     `json:"available"`
}

type subscriptionModelsDocument struct {
	Subscription  string          `json:"subscription"`
	Catalogue     string          `json:"catalogue"`
	Name          string          `json:"name"`
	SomethingLeft bool            `json:"something_left"`
	Models        []modelDocument `json:"models"`
}

// models lists the models of every subscription, the best ranked first,
// and which can be used with what is left; it never connects (ADR 0005).
func models(args []string, env environment) int {
	flags := newFlags("models", env)
	asJSON := flags.Bool("json", false, "print JSON")
	all := flags.Bool("all", false, "list every model, not only the first 15")
	if err := flags.Parse(args); err != nil || flags.NArg() > 1 {
		fmt.Fprintln(env.stderr, "usage: ataecina models [--json] [--all] [<subscription>]")
		return exitUsage
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: models: %v\n", err)
		return exitFailure
	}
	listing := offeringsapp.NewListModels(offeringsfile.NewStore(paths.StateDir()), offeringsfile.NewRankingStore(paths.StateDir())).List()
	left := somethingLeft(paths, env)
	documents := make([]subscriptionModelsDocument, 0, len(listing.Subscriptions))
	for _, subscription := range listing.Subscriptions {
		if flags.NArg() == 1 && subscription.Subscription != flags.Arg(0) {
			continue
		}
		documents = append(documents, describeModels(subscription, left[subscription.Subscription]))
	}
	if *asJSON {
		return writeJSON(env, map[string]any{"data": documents, "meta": map[string]any{
			"catalogue_refreshed_at":     instantOrNil(listing.CatalogueRefreshedAt),
			"intelligence_refreshed_at":  instantOrNil(listing.RankingRefreshedAt),
			"intelligence_index_version": stringOrNil(listing.RankingVersion),
			"intelligence_source":        "Artificial Analysis Intelligence Index",
			"intelligence_url":           "https://artificialanalysis.ai/",
		}})
	}
	writeModels(env, documents, *all)
	return exitOK
}

func describeModels(subscription offeringsapp.SubscriptionModels, left bool) subscriptionModelsDocument {
	document := subscriptionModelsDocument{
		Subscription: subscription.Subscription, Catalogue: subscription.Provider, Name: subscription.Name,
		SomethingLeft: left, Models: make([]modelDocument, 0, len(subscription.Models)),
	}
	for _, model := range subscription.Models {
		shown := modelDocument{ID: model.Model, Name: model.Name, Free: model.Free(), Available: left || model.Free()}
		if model.Priced {
			input, output := model.InputPrice, model.OutputPrice
			shown.InputPrice, shown.OutputPrice = &input, &output
		}
		if !model.ReleasedOn.IsZero() {
			day := model.ReleasedOn.Format(time.DateOnly)
			shown.ReleasedOn = &day
		}
		if model.HasIndex {
			index := model.Index
			shown.Intelligence = &index
		}
		document.Models = append(document.Models, shown)
	}
	return document
}

func writeModels(env environment, documents []subscriptionModelsDocument, all bool) {
	for _, document := range documents {
		fmt.Fprintf(env.stdout, "%s (%s), %d models\n", document.Subscription, document.Name, len(document.Models))
		table := tabwriter.NewWriter(env.stdout, 0, 4, 2, ' ', 0)
		for i, model := range document.Models {
			if i == 15 && !all {
				fmt.Fprintf(table, "  … %d more (--all)\t\t\t\n", len(document.Models)-15)
				break
			}
			index, price := "", ""
			if model.Intelligence != nil {
				index = strconv.FormatFloat(*model.Intelligence, 'f', 1, 64)
			}
			switch {
			case model.Free:
				price = "free"
			case model.InputPrice != nil:
				price = fmt.Sprintf("$%g / $%g per 1M", *model.InputPrice, *model.OutputPrice)
			}
			mark := ""
			if !model.Available {
				mark = "(nothing left)"
			}
			fmt.Fprintf(table, "  %s\t%s\t%s\t%s\n", model.Name, index, price, mark)
		}
		table.Flush()
	}
	fmt.Fprintln(env.stdout, "Intelligence: Artificial Analysis Intelligence Index (https://artificialanalysis.ai/)")
}

// somethingLeft tells, per subscription, whether its latest reading still
// lets paid models be used: it has figures, no window is used up and no
// money ran out. A failing source (a plan not subscribed, a key missing)
// leaves only the free models.
func somethingLeft(paths xdg.Paths, env environment) map[string]bool {
	left := map[string]bool{}
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return left
	}
	report, err := application.NewReportSnapshot(snapshotfile.NewStore(runtimeDir), env.clock).Report()
	if err != nil {
		return left
	}
	for _, status := range report.Statuses() {
		reading := status.Reading()
		left[reading.Subscription().String()] = reading.Outcome().HasFigures() && !usedUp(reading)
	}
	return left
}

func usedUp(reading domain.Reading) bool {
	for _, meter := range reading.Meters() {
		if meter.Used().Float64() >= 100 {
			return true
		}
	}
	if balance, ok := reading.Balance(); ok {
		if remaining, known := balance.Remaining(); known && remaining.Amount().Float64() <= 0 {
			return true
		}
	}
	return false
}

func instantOrNil(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return at.UTC().Format(time.RFC3339)
}

func stringOrNil(text string) any {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return text
}
