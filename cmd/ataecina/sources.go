package main

import (
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
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
	return sourceRegistry{catalog: catalog.Standard(), factories: map[domain.SourceID]sourceFactory{}}
}
