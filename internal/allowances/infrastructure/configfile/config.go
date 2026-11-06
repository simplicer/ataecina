// Package configfile reads $XDG_CONFIG_HOME/ataecina/config.json strictly:
// unknown keys, options a source does not take, bad values or loose
// permissions stop Ataecina before it does anything.
package configfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	formatVersion          = 1
	maxBytes               = 256 << 10
	defaultIntervalMinutes = 5
	maxIntervalMinutes     = 1440
	defaultAppServerAfter  = 60
	maxThresholds          = 5
)

var defaultThresholds = []int{90, 95}

// ErrInvalid reports a configuration Ataecina refuses.
var ErrInvalid = errors.New("invalid configuration")

// ErrMissing reports that config.json does not exist yet.
var ErrMissing = errors.New("no configuration")

// Settings is a validated configuration.
type Settings struct {
	Interval            time.Duration
	Thresholds          []int
	UseEnvironmentProxy bool
	Subscriptions       []SubscriptionSettings
}

// DomainSubscriptions lists the configured subscriptions in order.
func (s Settings) DomainSubscriptions() []domain.Subscription {
	subscriptions := make([]domain.Subscription, len(s.Subscriptions))
	for i, item := range s.Subscriptions {
		subscriptions[i] = item.Subscription
	}
	return subscriptions
}

// SubscriptionSettings is one configured subscription and its options.
type SubscriptionSettings struct {
	Subscription   domain.Subscription
	Key            string
	AppServer      bool
	AppServerAfter time.Duration
	// Thresholds are the subscription's own, or the global ones.
	Thresholds []int
}

type document struct {
	Version         *int                   `json:"version"`
	IntervalMinutes *int                   `json:"interval_minutes"`
	Alerts          *alertsDocument        `json:"alerts"`
	Network         *networkDocument       `json:"network"`
	Subscriptions   []subscriptionDocument `json:"subscriptions"`
}

type alertsDocument struct {
	Thresholds []int `json:"thresholds"`
}

type networkDocument struct {
	UseEnvironmentProxy bool `json:"use_environment_proxy"`
}

type subscriptionDocument struct {
	ID                    string  `json:"id"`
	Source                string  `json:"source"`
	Plan                  *string `json:"plan"`
	Provider              *string `json:"provider"`
	Key                   *string `json:"key"`
	AppServer             *bool   `json:"app_server"`
	AppServerAfterMinutes *int    `json:"app_server_after_minutes"`
	Thresholds            []int   `json:"thresholds"`
}

// Load reads config.json after checking that it and its directory are private.
func Load(path string, sources catalog.Catalog) (Settings, error) {
	if err := privatefs.RequireDir(filepath.Dir(path)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Settings{}, fmt.Errorf("%w: %s does not exist", ErrMissing, path)
		}
		return Settings{}, err
	}
	data, err := privatefs.ReadFile(path, maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return Settings{}, fmt.Errorf("%w: %s does not exist", ErrMissing, path)
	}
	if err != nil {
		return Settings{}, err
	}
	return Decode(data, sources)
}

// Decode validates the content of config.json.
func Decode(data []byte, sources catalog.Catalog) (Settings, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var raw document
	if err := decoder.Decode(&raw); err != nil {
		return Settings{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return Settings{}, fmt.Errorf("%w: unexpected data after the configuration object", ErrInvalid)
	}
	if raw.Version == nil || *raw.Version != formatVersion {
		return Settings{}, fmt.Errorf("%w: \"version\" must be %d", ErrInvalid, formatVersion)
	}
	settings := Settings{Interval: defaultIntervalMinutes * time.Minute, Thresholds: defaultThresholds}
	if raw.IntervalMinutes != nil {
		if *raw.IntervalMinutes < 1 || *raw.IntervalMinutes > maxIntervalMinutes {
			return Settings{}, fmt.Errorf("%w: \"interval_minutes\" must be between 1 and %d", ErrInvalid, maxIntervalMinutes)
		}
		settings.Interval = time.Duration(*raw.IntervalMinutes) * time.Minute
	}
	if raw.Alerts != nil {
		if err := validThresholds(raw.Alerts.Thresholds); err != nil {
			return Settings{}, err
		}
		settings.Thresholds = raw.Alerts.Thresholds
	}
	if raw.Network != nil {
		settings.UseEnvironmentProxy = raw.Network.UseEnvironmentProxy
	}
	seen := make(map[string]bool, len(raw.Subscriptions))
	for _, item := range raw.Subscriptions {
		subscription, err := decodeSubscription(item, sources)
		if err != nil {
			return Settings{}, err
		}
		if seen[item.ID] {
			return Settings{}, fmt.Errorf("%w: subscription %q appears twice", ErrInvalid, item.ID)
		}
		seen[item.ID] = true
		subscription.Thresholds = settings.Thresholds
		if item.Thresholds != nil {
			if err := validThresholds(item.Thresholds); err != nil {
				return Settings{}, fmt.Errorf("subscription %q: %w", item.ID, err)
			}
			subscription.Thresholds = item.Thresholds
		}
		settings.Subscriptions = append(settings.Subscriptions, subscription)
	}
	return settings, nil
}

func validThresholds(thresholds []int) error {
	if len(thresholds) == 0 || len(thresholds) > maxThresholds {
		return fmt.Errorf("%w: \"alerts.thresholds\" needs 1 to %d values", ErrInvalid, maxThresholds)
	}
	for i, threshold := range thresholds {
		if threshold < 1 || threshold > 100 || (i > 0 && threshold <= thresholds[i-1]) {
			return fmt.Errorf("%w: \"alerts.thresholds\" must rise strictly between 1 and 100", ErrInvalid)
		}
	}
	return nil
}

func decodeSubscription(item subscriptionDocument, sources catalog.Catalog) (SubscriptionSettings, error) {
	id, err := domain.NewSubscriptionID(item.ID)
	if err != nil {
		return SubscriptionSettings{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	source, err := domain.NewSourceID(item.Source)
	if err != nil {
		return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: %w", ErrInvalid, id, err)
	}
	entry, known := sources.Lookup(source)
	if !known {
		return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: unknown source %q", ErrInvalid, id, source)
	}
	options := []struct {
		name    catalog.Option
		present bool
	}{
		{catalog.OptionAppServer, item.AppServer != nil},
		{catalog.OptionAppServerAfter, item.AppServerAfterMinutes != nil},
		{catalog.OptionProvider, item.Provider != nil},
	}
	for _, option := range options {
		if option.present && !entry.Accepts(option.name) {
			return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: source %s does not take %q", ErrInvalid, id, source, option.name)
		}
	}
	if item.Key != nil && !entry.NeedsKey {
		return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: source %s needs no key", ErrInvalid, id, source)
	}
	provider := entry.Provider
	if item.Provider != nil {
		if provider, err = domain.NewProviderID(*item.Provider); err != nil {
			return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: %w", ErrInvalid, id, err)
		}
	}
	if provider == "" {
		return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: source %s needs \"provider\"", ErrInvalid, id, source)
	}
	settings := SubscriptionSettings{
		Subscription:   domain.NewSubscription(id, provider, source, entry.Fidelity, domain.NewPlan(valueOr(item.Plan, ""))),
		AppServer:      valueOr(item.AppServer, false),
		AppServerAfter: time.Duration(valueOr(item.AppServerAfterMinutes, defaultAppServerAfter)) * time.Minute,
	}
	if settings.AppServerAfter < time.Minute {
		return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: \"app_server_after_minutes\" must be at least 1", ErrInvalid, id)
	}
	if entry.NeedsKey {
		settings.Key = valueOr(item.Key, id.String())
		if _, err := domain.NewSubscriptionID(settings.Key); err != nil {
			return SubscriptionSettings{}, fmt.Errorf("%w: subscription %q: key name %q (lowercase letters, digits and dashes)", ErrInvalid, id, settings.Key)
		}
	}
	return settings, nil
}

func valueOr[T any](pointer *T, fallback T) T {
	if pointer == nil {
		return fallback
	}
	return *pointer
}
