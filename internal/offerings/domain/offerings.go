// Package domain is the model catalogue (SPECS §11, ADR 0015): which
// models each provider offers, at what list price, which of them are free
// and which are new. It knows nothing of quotas, keys or where the
// catalogue comes from.
package domain

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"
)

// ErrNoCatalogue is a catalogue that was never fetched.
var ErrNoCatalogue = errors.New("no model catalogue yet")

const (
	// refreshEvery is how old a catalogue may grow before it is asked again.
	refreshEvery = 24 * time.Hour
	// retryAfter is how long a failed refresh waits before trying again.
	retryAfter = time.Hour
	// newFor is how long a released model counts as new.
	newFor = 30 * 24 * time.Hour
)

// Offering is one model of a provider: its id, its display name, its list
// price in USD per million tokens (Priced is false when the catalogue
// gives none) and the day it was released (zero when unknown).
type Offering struct {
	Model       string
	Name        string
	Priced      bool
	InputPrice  float64
	OutputPrice float64
	ReleasedOn  time.Time
}

// Free reports a model priced at no cost, for input and output alike; a
// model without a price is not free, its price is unknown.
func (o Offering) Free() bool { return o.Priced && o.InputPrice == 0 && o.OutputPrice == 0 }

// NewAt reports a model released within the last 30 days.
func (o Offering) NewAt(now time.Time) bool {
	return !o.ReleasedOn.IsZero() && !o.ReleasedOn.After(now) && now.Sub(o.ReleasedOn) <= newFor
}

// ProviderOfferings are the models one provider of the catalogue offers.
type ProviderOfferings struct {
	Provider string
	Name     string
	Models   []Offering
}

// Free lists the free models, by name.
func (p ProviderOfferings) Free() []Offering {
	var free []Offering
	for _, model := range p.Models {
		if model.Free() {
			free = append(free, model)
		}
	}
	return byName(free)
}

// NewAt lists the models released within the last 30 days, newest first.
func (p ProviderOfferings) NewAt(now time.Time) []Offering {
	var fresh []Offering
	for _, model := range p.Models {
		if model.NewAt(now) {
			fresh = append(fresh, model)
		}
	}
	slices.SortStableFunc(fresh, func(a, b Offering) int { return b.ReleasedOn.Compare(a.ReleasedOn) })
	return fresh
}

func byName(models []Offering) []Offering {
	slices.SortStableFunc(models, func(a, b Offering) int { return cmp.Compare(a.Name, b.Name) })
	return models
}

// Interest is one subscription whose models one provider of the catalogue
// describes.
type Interest struct {
	Subscription string
	Provider     string
}

// Catalogue is the part of the catalogue Ataecina keeps: the interests it
// was kept for, when it was last checked and last refreshed, and the
// validator that lets the next check skip an unchanged catalogue.
type Catalogue struct {
	Interests   []Interest
	CheckedAt   time.Time
	RefreshedAt time.Time
	Validator   string
	Providers   []ProviderOfferings
}

// ProvidersOf lists the providers of some interests, once each, sorted.
func ProvidersOf(interests []Interest) []string {
	providers := make([]string, 0, len(interests))
	for _, interest := range interests {
		providers = append(providers, interest.Provider)
	}
	slices.Sort(providers)
	return slices.Compact(providers)
}

// DueAt tells whether the catalogue must be asked again: never fetched,
// kept for other providers, a day old, or an hour after a failed check.
func (c Catalogue) DueAt(now time.Time, interests []Interest) bool {
	switch {
	case c.CheckedAt.IsZero():
		return true
	case !slices.Equal(ProvidersOf(c.Interests), ProvidersOf(interests)):
		return true
	case c.CheckedAt.After(c.RefreshedAt):
		return now.Sub(c.CheckedAt) >= retryAfter
	}
	return now.Sub(c.RefreshedAt) >= refreshEvery
}

// ValidatorFor is the validator to send when asking for the providers of
// these interests: only a catalogue already refreshed for exactly them can
// be confirmed as unchanged (the validator covers the whole source, not
// the part kept).
func (c Catalogue) ValidatorFor(interests []Interest) string {
	if c.RefreshedAt.IsZero() || !slices.Equal(ProvidersOf(c.Interests), ProvidersOf(interests)) {
		return ""
	}
	return c.Validator
}

// For keeps the same offerings for other interests on the same providers
// (a subscription renamed, or one more on a provider already kept).
func (c Catalogue) For(interests []Interest) Catalogue {
	c.Interests = sortedInterests(interests)
	return c
}

// Provider finds one provider's offerings.
func (c Catalogue) Provider(id string) (ProviderOfferings, bool) {
	for _, provider := range c.Providers {
		if provider.Provider == id {
			return provider, true
		}
	}
	return ProviderOfferings{}, false
}

// Summary is what the displays show of one subscription's provider: how
// many models it offers, which are free and which are new.
type Summary struct {
	Subscription string
	Provider     string
	Name         string
	Models       int
	Free         []Offering
	New          []Offering
}

// Summaries summarises the provider of each interest the catalogue holds,
// in the order of the interests.
func (c Catalogue) Summaries(now time.Time) []Summary {
	var summaries []Summary
	for _, interest := range c.Interests {
		offerings, ok := c.Provider(interest.Provider)
		if !ok {
			continue
		}
		summaries = append(summaries, Summary{
			Subscription: interest.Subscription, Provider: offerings.Provider, Name: offerings.Name,
			Models: len(offerings.Models), Free: offerings.Free(), New: offerings.NewAt(now),
		})
	}
	return summaries
}

func sortedInterests(interests []Interest) []Interest {
	sorted := slices.Clone(interests)
	slices.SortFunc(sorted, func(a, b Interest) int { return cmp.Compare(a.Subscription, b.Subscription) })
	return sorted
}

// Fetched is the answer of the catalogue's source: unchanged since the
// validator, or the offerings of the providers asked for.
type Fetched struct {
	NotModified bool
	Validator   string
	Providers   []ProviderOfferings
}

// Refreshed is the catalogue after a successful check.
func (c Catalogue) Refreshed(fetched Fetched, interests []Interest, now time.Time) Catalogue {
	next := Catalogue{Interests: sortedInterests(interests), CheckedAt: now, RefreshedAt: now, Validator: fetched.Validator, Providers: fetched.Providers}
	if fetched.NotModified {
		next.Providers = c.Providers
		if fetched.Validator == "" {
			next.Validator = c.Validator
		}
	}
	return next
}

// Failed is the catalogue after a failed check: the offerings already
// known stay, and the next check waits an hour.
func (c Catalogue) Failed(now time.Time) Catalogue {
	c.CheckedAt = now
	return c
}

// OfferingsSource fetches the offerings of some providers; with a
// validator it may answer that nothing changed.
type OfferingsSource interface {
	Fetch(ctx context.Context, providers []string, validator string) (Fetched, error)
}

// Catalogues keeps the catalogue between collections.
type Catalogues interface {
	Load() (Catalogue, error)
	Save(Catalogue) error
}

// Clock gives the current time.
type Clock interface {
	Now() time.Time
}
