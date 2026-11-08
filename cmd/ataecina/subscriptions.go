package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	keysapp "codeberg.org/simplicer/ataecina/internal/keys/application"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

const subscriptionsUsage = `usage: ataecina subscriptions [--json]
       ataecina subscriptions add <id> --source <source> [--plan <plan>] [--key <name>] [--set <option>=<value>]…
       ataecina subscriptions remove <id>`

// sourceDocument is one source of `ataecina sources --json`.
type sourceDocument struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Provider          *string  `json:"provider"`
	Fidelity          string   `json:"fidelity"`
	NeedsKey          bool     `json:"needs_key"`
	Options           []string `json:"options"`
	Required          []string `json:"required"`
	TakesDeclarations bool     `json:"takes_declarations"`
}

// sourcesCommand lists the sources Ataecina knows and the options each
// takes; the widget's Subscriptions page reads it.
func sourcesCommand(args []string, env environment) int {
	flags := newFlags("sources", env)
	asJSON := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	documents := make([]sourceDocument, 0)
	for _, entry := range env.sources.catalog.Entries() {
		document := sourceDocument{
			ID: entry.Source.String(), Label: entry.Source.Label(), Fidelity: string(entry.Fidelity), NeedsKey: entry.NeedsKey,
			Options: optionNames(entry.Options), Required: optionNames(entry.Requires), TakesDeclarations: entry.TakesDeclarations,
		}
		if entry.Provider != "" {
			provider := entry.Provider.String()
			document.Provider = &provider
		} else {
			// Heimdall follows a provider the owner names.
			document.Required = append(document.Required, string(catalog.OptionProvider))
		}
		documents = append(documents, document)
	}
	if *asJSON {
		return writeJSON(env, map[string]any{"data": documents})
	}
	table := tabwriter.NewWriter(env.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "SOURCE\tPROVIDER\tKEY\tREQUIRED OPTIONS")
	for _, document := range documents {
		provider := "(declared)"
		if document.Provider != nil {
			provider = *document.Provider
		}
		key := "no"
		if document.NeedsKey {
			key = "yes"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", document.ID, provider, key, strings.Join(document.Required, ", "))
	}
	table.Flush()
	return exitOK
}

func optionNames(options []catalog.Option) []string {
	names := make([]string, 0, len(options))
	for _, option := range options {
		names = append(names, string(option))
	}
	return names
}

// subscriptionsCommand lists, adds or removes the subscriptions of
// config.json; every change is validated like `collect` validates.
func subscriptionsCommand(args []string, env environment) int {
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: subscriptions: %v\n", err)
		return exitFailure
	}
	if len(args) > 0 && args[0] == "add" {
		return addSubscription(args[1:], paths, env)
	}
	if len(args) > 0 && args[0] == "remove" {
		return removeSubscription(args[1:], paths, env)
	}
	return listSubscriptions(args, paths, env)
}

type subscriptionDocument struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`
	Label    string         `json:"label"`
	Provider string         `json:"provider"`
	Plan     *string        `json:"plan"`
	Key      *string        `json:"key"`
	Options  map[string]any `json:"options"`
}

func listSubscriptions(args []string, paths xdg.Paths, env environment) int {
	flags := newFlags("subscriptions", env)
	asJSON := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		fmt.Fprintln(env.stderr, subscriptionsUsage)
		return exitUsage
	}
	settings, err := configfile.Load(paths.ConfigFile(), env.sources.catalog)
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: subscriptions: %v\n", err)
		return exitFailure
	}
	documents := make([]subscriptionDocument, 0, len(settings.Subscriptions))
	for _, item := range settings.Subscriptions {
		documents = append(documents, describeSubscription(item))
	}
	if *asJSON {
		return writeJSON(env, map[string]any{"data": documents})
	}
	table := tabwriter.NewWriter(env.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSOURCE\tPLAN\tKEY")
	for _, document := range documents {
		plan, key := "", ""
		if document.Plan != nil {
			plan = *document.Plan
		}
		if document.Key != nil {
			key = *document.Key
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", document.ID, document.Source, plan, key)
	}
	table.Flush()
	return exitOK
}

// describeSubscription shows the options a subscription sets, never a
// secret (config.json holds none).
func describeSubscription(item configfile.SubscriptionSettings) subscriptionDocument {
	subscription := item.Subscription
	document := subscriptionDocument{
		ID: subscription.ID().String(), Source: subscription.Source().String(), Label: subscription.Source().Label(),
		Provider: subscription.Provider().String(), Options: map[string]any{},
	}
	if plan := subscription.Plan().String(); plan != "" {
		document.Plan = &plan
	}
	if item.Key != "" {
		key := item.Key
		document.Key = &key
	}
	for name, value := range map[string]string{"account": item.Account, "login": item.Login, "team": item.Team, "email": item.Email, "catalogue": item.Catalogue} {
		if value != "" {
			document.Options[name] = value
		}
	}
	if item.MonthlyAllowance > 0 {
		document.Options["monthly_allowance"] = item.MonthlyAllowance
	}
	if item.Port > 0 {
		document.Options["port"] = item.Port
	}
	if item.AppServer {
		document.Options["app_server"] = true
	}
	return document
}

// optionValues collects repeated --set name=value flags.
type optionValues map[string]string

func (o optionValues) String() string { return "" }

func (o optionValues) Set(value string) error {
	name, text, found := strings.Cut(value, "=")
	if !found || name == "" {
		return errors.New("use --set <option>=<value>")
	}
	o[name] = text
	return nil
}

func addSubscription(args []string, paths xdg.Paths, env environment) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(env.stderr, subscriptionsUsage)
		return exitUsage
	}
	id := args[0]
	flags := newFlags("subscriptions add", env)
	source := flags.String("source", "", "source id (see `ataecina sources`)")
	plan := flags.String("plan", "", "plan name, when the source does not report it")
	key := flags.String("key", "", "sealed key name, default the subscription id")
	options := optionValues{}
	flags.Var(options, "set", "a source option, as name=value (repeatable)")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() > 0 || *source == "" {
		fmt.Fprintln(env.stderr, subscriptionsUsage)
		return exitUsage
	}
	err := configfile.AddSubscription(paths.ConfigFile(), env.sources.catalog, configfile.NewSubscription{
		ID: id, Source: *source, Plan: *plan, Key: *key, Options: options,
	})
	if errors.Is(err, configfile.ErrSubscriptionExists) {
		fmt.Fprintf(env.stderr, "ataecina: subscriptions add: %s already exists; remove it first with `ataecina subscriptions remove %s`, or choose another id\n", id, id)
		return exitFailure
	}
	if err != nil {
		fmt.Fprintf(env.stderr, "ataecina: subscriptions add: %v\n", err)
		return exitFailure
	}
	fmt.Fprintf(env.stdout, "added %s\n", id)
	if entry, ok := env.sources.catalog.Lookup(domain.SourceID(*source)); ok && entry.NeedsKey {
		name := *key
		if name == "" {
			name = id
		}
		if sealedKeys(paths)[name] {
			fmt.Fprintf(env.stdout, "its key %s is already sealed\n", name)
		} else {
			fmt.Fprintf(env.stdout, "set its key with: ataecina keys set %s\n", name)
		}
	}
	return exitOK
}

// sealedKeys names the keys in keys.json, without unsealing any; none
// when the file cannot be read.
func sealedKeys(paths xdg.Paths) map[string]bool {
	keys, _ := keysapp.NewListKeys(keysFiles(paths)).List()
	sealed := make(map[string]bool, len(keys))
	for _, key := range keys {
		sealed[key.Name().String()] = true
	}
	return sealed
}

func removeSubscription(args []string, paths xdg.Paths, env environment) int {
	if len(args) != 1 {
		fmt.Fprintln(env.stderr, subscriptionsUsage)
		return exitUsage
	}
	if err := configfile.RemoveSubscription(paths.ConfigFile(), env.sources.catalog, args[0]); err != nil {
		fmt.Fprintf(env.stderr, "ataecina: subscriptions remove: %v\n", err)
		return exitFailure
	}
	fmt.Fprintf(env.stdout, "removed %s (its sealed key, if any, stays until `ataecina keys forget`)\n", args[0])
	return exitOK
}

func writeJSON(env environment, value any) int {
	encoder := json.NewEncoder(env.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return exitFailure
	}
	return exitOK
}
