package main

import (
	"context"
	"fmt"
	"strings"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/balancefile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

const balanceUsage = "usage: ataecina balance declare <subscription> (--top-up <amount> | --balance <amount>) [--currency USD]"

// balance records a top-up or the current balance of a balance-only
// subscription (`opencode-zen`), with the spending to date as baseline.
func balance(ctx context.Context, args []string, env environment) int {
	if len(args) < 2 || args[0] != "declare" || strings.HasPrefix(args[1], "-") {
		fmt.Fprintln(env.stderr, "ataecina: balance: "+balanceUsage)
		return exitUsage
	}
	flags := newFlags("balance declare", env)
	topUp := flags.String("top-up", "", "money added to the balance")
	current := flags.String("balance", "", "the whole current balance, replacing earlier declarations")
	code := flags.String("currency", "USD", "ISO 4217 currency")
	if err := flags.Parse(args[2:]); err != nil || flags.NArg() > 0 || (*topUp == "") == (*current == "") {
		fmt.Fprintln(env.stderr, "ataecina: balance: "+balanceUsage)
		return exitUsage
	}
	kind, raw := domain.DeclaredTopUp, *topUp
	if *current != "" {
		kind, raw = domain.DeclaredBalance, *current
	}
	declaration, err := declareBalance(ctx, args[1], kind, raw, *code, env)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: balance: %v\n", err)
		return exitFailure
	}
	fmt.Fprintf(env.stdout, "%s: declared %s %s %s (spent to date: %s %s)\n", args[1],
		strings.ToLower(strings.ReplaceAll(string(declaration.Kind()), "_", "-")),
		declaration.Amount().Amount(), declaration.Amount().Currency(),
		declaration.SpentBaseline().Amount(), declaration.SpentBaseline().Currency())
	return exitOK
}

func declareBalance(ctx context.Context, id string, kind domain.DeclarationKind, raw, code string, env environment) (domain.Declaration, error) {
	subscription, err := domain.NewSubscriptionID(id)
	if err != nil {
		return domain.Declaration{}, err
	}
	amount, err := domain.ParseAmount(raw)
	if err != nil {
		return domain.Declaration{}, err
	}
	currency, err := domain.NewCurrency(code)
	if err != nil {
		return domain.Declaration{}, err
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return domain.Declaration{}, err
	}
	settings, err := configfile.Load(paths.ConfigFile(), env.sources.catalog)
	if err != nil {
		return domain.Declaration{}, err
	}
	if err := takesDeclarations(settings, subscription, env); err != nil {
		return domain.Declaration{}, err
	}
	declare := application.NewDeclareBalance(balancefile.NewStore(paths.StateDir()), zenSpend(env), env.clock)
	return declare.Declare(ctx, subscription, kind, domain.NewMoney(amount, currency))
}

// takesDeclarations checks that the subscription is configured with a
// balance-only source.
func takesDeclarations(settings configfile.Settings, id domain.SubscriptionID, env environment) error {
	for _, item := range settings.Subscriptions {
		if item.Subscription.ID() != id {
			continue
		}
		if entry, _ := env.sources.catalog.Lookup(item.Subscription.Source()); !entry.TakesDeclarations {
			return fmt.Errorf("subscription %q (%s) does not take declared balances; only opencode-zen does", id, item.Subscription.Source())
		}
		return nil
	}
	return fmt.Errorf("subscription %q is not in config.json", id)
}
