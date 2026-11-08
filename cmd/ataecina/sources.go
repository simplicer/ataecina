package main

import (
	"context"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/alibabacli"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/balancefile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/claudestatusline"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cloudflareapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexappserver"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexlogs"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/commandcodeapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/copilotgh"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cursoradminapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/heimdallplans"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/kimicodeapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/minimaxapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/moonshotapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/ollamaapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodeconsole"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodezen"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/openrouterapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/orcarouterapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/vercelapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/xaiapi"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/zaiapi"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/pathlookup"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// collection is what every adapter of one collection may use.
type collection struct {
	paths xdg.Paths
	env   environment
	keys  domain.KeyRing
	http  httpclient.Client
}

// sourceFactory builds the adapter of one configured subscription.
type sourceFactory func(settings configfile.SubscriptionSettings, c collection) domain.Source

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

func (r sourceRegistry) build(settings configfile.Settings, c collection) subscriptionSources {
	sources := make(subscriptionSources, len(settings.Subscriptions))
	for _, item := range settings.Subscriptions {
		if factory, ok := r.factories[item.Subscription.Source()]; ok {
			sources[item.Subscription.ID()] = factory(item, c)
		}
	}
	return sources
}

// viaAPI builds a provider API adapter bound to its compiled-in origin.
func viaAPI[S domain.Source](origin string, build func(httpclient.Endpoint, domain.KeyRing, domain.KeyName, func() time.Time) S) sourceFactory {
	return viaEndpoint(origin, func(endpoint httpclient.Endpoint, settings configfile.SubscriptionSettings, c collection) domain.Source {
		return build(endpoint, c.keys, domain.KeyName(settings.Key), c.env.clock.Now)
	})
}

// viaEndpoint is viaAPI for adapters that also need the subscription's options.
func viaEndpoint(origin string, build func(httpclient.Endpoint, configfile.SubscriptionSettings, collection) domain.Source) sourceFactory {
	return func(settings configfile.SubscriptionSettings, c collection) domain.Source {
		endpoint, err := c.http.For(providerOrigin(c.env, origin))
		if err != nil {
			return failing{domain.Failure(domain.ErrorContractChanged, "the provider origin is invalid", err)}
		}
		return build(endpoint, settings, c)
	}
}

// failing is a source that can only fail.
type failing struct{ failure *domain.SourceFailure }

func (f failing) Read(context.Context, domain.Subscription) (domain.Reading, error) {
	return domain.Reading{}, f.failure
}

// standardSources is the production registry.
func standardSources() sourceRegistry {
	return sourceRegistry{catalog: catalog.Standard(), factories: map[domain.SourceID]sourceFactory{
		codexlogs.SourceID: func(settings configfile.SubscriptionSettings, c collection) domain.Source {
			logs := codexlogs.New(codexHome(c.paths, c.env), filepath.Join(c.paths.StateDir(), "codex-logs"), c.env.clock.Now)
			if !settings.AppServer {
				return logs
			}
			return codexappserver.NewFallback(logs, appServer(c.env), settings.AppServerAfter, c.env.clock.Now)
		},
		codexappserver.SourceID: func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			return appServer(c.env)
		},
		opencodezen.SourceID: func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			return opencodezen.NewSource(balancefile.NewStore(c.paths.StateDir()), zenSpend(c.env), c.env.clock.Now)
		},
		opencodeconsole.SourceID: func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			return opencodeconsole.New(pathlookup.Find("opencode", c.env.getenv("PATH")), c.env.clock.Now)
		},
		copilotgh.SourceID: func(settings configfile.SubscriptionSettings, c collection) domain.Source {
			return copilotgh.New(pathlookup.Find("gh", c.env.getenv("PATH")), settings.Login, settings.MonthlyAllowance, c.env.clock.Now)
		},
		heimdallplans.SourceID: func(settings configfile.SubscriptionSettings, c collection) domain.Source {
			// Heimdall is a local service: loopback only, the owner chooses the port (ADR 0013).
			endpoint, err := c.http.ForLoopback(settings.Port)
			if err != nil {
				return failing{domain.Failure(domain.ErrorContractChanged, "the Heimdall port is invalid", err)}
			}
			return heimdallplans.New(endpoint, c.keys, domain.KeyName(settings.Key), c.env.clock.Now)
		},
		alibabacli.SourceID: func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			return alibabacli.New(pathlookup.Find("bl", c.env.getenv("PATH")), c.env.clock.Now)
		},
		claudestatusline.SourceID: func(_ configfile.SubscriptionSettings, c collection) domain.Source {
			// collect has already required the runtime directory.
			runtimeDir, _ := c.paths.RuntimeDir()
			return claudestatusline.NewSource(filepath.Join(runtimeDir, "signals"))
		},
		"openrouter-api":  viaAPI(openrouterapi.Origin, openrouterapi.New),
		"vercel-api":      viaAPI(vercelapi.Origin, vercelapi.New),
		"zai-api":         viaAPI(zaiapi.Origin, zaiapi.New),
		"minimax-api":     viaAPI(minimaxapi.Origin, minimaxapi.New),
		"moonshot-api":    viaAPI(moonshotapi.Global.Origin, moonshotapi.New(moonshotapi.Global)),
		"moonshot-cn-api": viaAPI(moonshotapi.China.Origin, moonshotapi.New(moonshotapi.China)),
		"kimi-code-api":   viaAPI(kimicodeapi.Origin, kimicodeapi.New),
		"xai-api": viaEndpoint(xaiapi.Origin, func(endpoint httpclient.Endpoint, settings configfile.SubscriptionSettings, c collection) domain.Source {
			return xaiapi.New(endpoint, c.keys, domain.KeyName(settings.Key), settings.Team, c.env.clock.Now)
		}),
		"cursor-admin-api": viaEndpoint(cursoradminapi.Origin, func(endpoint httpclient.Endpoint, settings configfile.SubscriptionSettings, c collection) domain.Source {
			return cursoradminapi.New(endpoint, c.keys, domain.KeyName(settings.Key), settings.Email, c.env.clock.Now)
		}),
		"commandcode-api": viaAPI(commandcodeapi.Origin, commandcodeapi.New),
		"ollama-api":      viaAPI(ollamaapi.Origin, ollamaapi.New),
		"orcarouter-api":  viaAPI(orcarouterapi.Origin, orcarouterapi.New),
		"cloudflare-api": viaEndpoint(cloudflareapi.Origin, func(endpoint httpclient.Endpoint, settings configfile.SubscriptionSettings, c collection) domain.Source {
			return cloudflareapi.New(endpoint, c.keys, domain.KeyName(settings.Key), settings.Account, c.env.clock.Now)
		}),
	}}
}

// zenSpend reads OpenCode Zen spending with the `opencode` found on PATH.
func zenSpend(env environment) opencodezen.Stats {
	return opencodezen.NewStats(pathlookup.Find("opencode", env.getenv("PATH")))
}

// appServer runs the official `codex` found on the collector's PATH.
func appServer(env environment) codexappserver.Source {
	return codexappserver.New(pathlookup.Find("codex", env.getenv("PATH")), version, env.clock.Now)
}

// codexHome is $CODEX_HOME when absolute, as Codex itself reads it, else ~/.codex.
func codexHome(paths xdg.Paths, env environment) string {
	if home := env.getenv("CODEX_HOME"); filepath.IsAbs(home) {
		return filepath.Clean(home)
	}
	return filepath.Join(paths.Home(), ".codex")
}
