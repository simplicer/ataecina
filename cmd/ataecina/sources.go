package main

import (
	"path/filepath"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexappserver"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlogs"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// sourceFactory builds the adapter of one configured subscription.
type sourceFactory func(settings configfile.SubscriptionSettings, paths xdg.Paths, env environment) domain.Source

// sourceRegistry knows every source id (the catalog) and builds the
// adapters this binary implements; the others read as UNSUPPORTED.
type sourceRegistry struct {
	catalog   catalog.Catalog
	factories map[domain.SourceID]sourceFactory
}

func (r sourceRegistry) with(entry catalog.Entry, factory sourceFactory) sourceRegistry {
	factories := make(map[domain.SourceID]sourceFactory, len(r.factories)+1)
	for id, existing := range r.factories {
		factories[id] = existing
	}
	factories[entry.Source] = factory
	return sourceRegistry{catalog: r.catalog.With(entry), factories: factories}
}

// subscriptionSources is the domain.SourceCatalog of one configuration.
type subscriptionSources map[domain.SubscriptionID]domain.Source

func (s subscriptionSources) SourceOf(subscription domain.Subscription) (domain.Source, bool) {
	source, ok := s[subscription.ID()]
	return source, ok
}

func (r sourceRegistry) build(settings configfile.Settings, paths xdg.Paths, env environment) subscriptionSources {
	sources := make(subscriptionSources, len(settings.Subscriptions))
	for _, item := range settings.Subscriptions {
		if factory, ok := r.factories[item.Subscription.Source()]; ok {
			sources[item.Subscription.ID()] = factory(item, paths, env)
		}
	}
	return sources
}

// standardSources is the production registry.
func standardSources() sourceRegistry {
	return sourceRegistry{catalog: catalog.Standard(), factories: map[domain.SourceID]sourceFactory{
		codexlogs.SourceID: func(settings configfile.SubscriptionSettings, paths xdg.Paths, env environment) domain.Source {
			logs := codexlogs.New(codexHome(paths, env), filepath.Join(paths.StateDir(), "codex-logs"), env.clock.Now)
			if !settings.AppServer {
				return logs
			}
			return codexappserver.NewFallback(logs, appServer(env), settings.AppServerAfter, env.clock.Now)
		},
		codexappserver.SourceID: func(_ configfile.SubscriptionSettings, _ xdg.Paths, env environment) domain.Source {
			return appServer(env)
		},
	}}
}

// appServer runs the official `codex` found on the collector's PATH.
func appServer(env environment) codexappserver.Source {
	return codexappserver.New(codexappserver.FindOnPath(env.getenv("PATH")), version, env.clock.Now)
}

// codexHome is $CODEX_HOME when absolute, as Codex itself reads it, else ~/.codex.
func codexHome(paths xdg.Paths, env environment) string {
	if home := env.getenv("CODEX_HOME"); filepath.IsAbs(home) {
		return filepath.Clean(home)
	}
	return filepath.Join(paths.Home(), ".codex")
}
