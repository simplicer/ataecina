package main

import (
	"context"
	"log/slog"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/statusjson"
	offeringsapp "codeberg.org/simplicer/ataecina/internal/offerings/application"
	offeringsdomain "codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/modelsdev"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/offeringsfile"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// refreshOfferings keeps the model catalogue of the configured
// subscriptions, at most once a day, after the readings are collected; a
// failure is only logged (SPECS §11, ADR 0015).
func refreshOfferings(ctx context.Context, settings configfile.Settings, paths xdg.Paths, env environment, logger *slog.Logger) {
	interests := catalogueInterests(settings)
	client := httpclient.New(httpclient.Options{
		Version: version, UseEnvironmentProxy: settings.UseEnvironmentProxy, AllowPlainHTTP: plainHTTPAllowed,
	})
	endpoint, err := client.For(providerOrigin(env, modelsdev.Origin))
	if err != nil {
		logger.Error("model catalogue", slog.String("error", err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(ctx, modelsdev.Timeout+time.Second)
	defer cancel()
	refresh := offeringsapp.NewRefreshOfferings(modelsdev.New(endpoint), offeringsfile.NewStore(paths.StateDir()), env.clock)
	asked, err := refresh.Refresh(ctx, interests)
	switch {
	case err != nil:
		logger.Warn("model catalogue", slog.Bool("asked", asked), slog.String("error", err.Error()))
	case asked:
		logger.Info("model catalogue refreshed", slog.Int("subscriptions", len(interests)))
	}
}

// catalogueInterests names the catalogue provider of each subscription
// that has one.
func catalogueInterests(settings configfile.Settings) []offeringsdomain.Interest {
	var interests []offeringsdomain.Interest
	for _, item := range settings.Subscriptions {
		subscription := item.Subscription
		provider := modelsdev.ProviderFor(subscription.Source().String(), subscription.Provider().String(), item.Catalogue)
		if provider != "" {
			interests = append(interests, offeringsdomain.Interest{Subscription: subscription.ID().String(), Provider: provider})
		}
	}
	return interests
}

// offeringsOf reads the kept catalogue for `status --json`; it never asks
// models.dev (ADR 0005).
func offeringsOf(paths xdg.Paths, env environment) (map[string]statusjson.Offerings, time.Time) {
	summaries, refreshedAt := offeringsapp.NewReportOfferings(offeringsfile.NewStore(paths.StateDir()), env.clock).Report()
	shown := make(map[string]statusjson.Offerings, len(summaries))
	for _, summary := range summaries {
		shown[summary.Subscription] = statusjson.Offerings{
			Catalogue: summary.Provider, Name: summary.Name, Models: summary.Models,
			FreeModels: shownModels(summary.Free), NewModels: shownModels(summary.New),
		}
	}
	return shown, refreshedAt
}

func shownModels(models []offeringsdomain.Offering) []statusjson.Model {
	shown := make([]statusjson.Model, 0, len(models))
	for _, model := range models {
		shown = append(shown, statusjson.Model{ID: model.Model, Name: model.Name, ReleasedOn: model.ReleasedOn})
	}
	return shown
}
