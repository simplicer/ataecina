package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	alertsapp "codeberg.org/simplicer/ataecina/internal/alerts/application"
	alertsdomain "codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/alertfile"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify"
	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// raiseAlerts hands the collected meters to the alerts context (ADR 0002:
// alerts read meters, never sources) and logs what it announced.
func raiseAlerts(ctx context.Context, snapshot domain.Snapshot, settings configfile.Settings, paths xdg.Paths, env environment, logger *slog.Logger) {
	notifier := alertsdomain.Notifier(noDesktop{})
	if address := sessionBusAddress(env); address != "" {
		notifier = desktopnotify.New(func() (*dbus.Conn, error) { return dbus.Connect(address) }, env.clock.Now)
	}
	raise := alertsapp.NewRaiseThresholdAlerts(alertfile.NewStore(paths.StateDir()), notifier, env.clock)
	alerts, err := raise.Raise(ctx, watchedMeters(snapshot, settings, env))
	for _, alert := range alerts {
		logger.Info("alert raised",
			slog.String("subscription", alert.Instance.Subscription),
			slog.String("window", alert.Instance.Window),
			slog.Int("threshold", int(alert.Threshold)),
			slog.Float64("used_percent", alert.UsedPercent))
	}
	if err != nil {
		logger.Warn("alerts", slog.String("error", err.Error()))
	}
}

// watchedMeters are the meters with figures, as they stand now: a reborn
// window is not watched until a new reading arrives.
func watchedMeters(snapshot domain.Snapshot, settings configfile.Settings, env environment) []alertsdomain.WatchedMeter {
	thresholds := map[domain.SubscriptionID][]alertsdomain.Threshold{}
	for _, item := range settings.Subscriptions {
		for _, percent := range item.Thresholds {
			if threshold, err := alertsdomain.NewThreshold(percent); err == nil {
				thresholds[item.Subscription.ID()] = append(thresholds[item.Subscription.ID()], threshold)
			}
		}
	}
	var watched []alertsdomain.WatchedMeter
	for _, status := range snapshot.StatusAt(env.clock.Now()).Statuses() {
		reading := status.Reading()
		if !reading.Outcome().HasFigures() {
			continue
		}
		for _, meter := range reading.Meters() {
			if meter.Estimated() {
				continue
			}
			resetsAt, _ := meter.ResetsAt()
			window := string(meter.Window().Kind())
			if meter.Window().Kind() == domain.WindowOther {
				window = fmt.Sprintf("%s_%d_MINUTES", window, meter.Window().Minutes())
			}
			watched = append(watched, alertsdomain.WatchedMeter{
				Instance: alertsdomain.WindowInstance{
					Subscription: reading.Subscription().String(),
					Bucket:       meter.Bucket().String(),
					Window:       window,
					ResetsAt:     resetsAt,
				},
				UsedPercent: meter.Used().Float64(),
				Thresholds:  thresholds[reading.Subscription()],
			})
		}
	}
	return watched
}

// noDesktop is the notifier where there is no session bus (SSH, a
// headless collector): alerts are kept and logged, not shown.
type noDesktop struct{}

func (noDesktop) Notify(context.Context, alertsdomain.Alert) error {
	return errors.New("no session bus to show the alert on")
}
