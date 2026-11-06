package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/allowances/application"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/snapshotfile"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/tray"
	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// trayReadEvery is how often the tray reads the snapshot again; countdowns
// are shown in minutes.
const trayReadEvery = 30 * time.Second

// trayCommand shows the snapshot in the system tray until Quit, SIGINT or
// SIGTERM. Like status it never collects or unseals; "Refresh now" asks
// systemd to start the collector.
func trayCommand(ctx context.Context, args []string, env environment) int {
	flags := newFlags("tray", env)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return exitUsage
	}
	fail := func(err error) int {
		fmt.Fprintf(env.stderr, "ataecina: tray: %v\n", err)
		return exitFailure
	}
	paths, err := xdg.Resolve(env.getenv)
	if err != nil {
		return fail(err)
	}
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return fail(err)
	}
	address := sessionBusAddress(env)
	if address == "" {
		return fail(errors.New("no session bus: run it inside your desktop session"))
	}
	conn, err := dbus.Connect(address)
	if err != nil {
		return fail(fmt.Errorf("connect to the session bus: %w", err))
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	report := application.NewReportSnapshot(snapshotfile.NewStore(runtimeDir), env.clock)
	err = tray.New(tray.Options{
		Report: report.Report,
		Refresh: func(ctx context.Context) error {
			return systemdunit.Start(ctx, conn, "ataecina-collect.service")
		},
		Now:      env.clock.Now,
		Every:    trayReadEvery,
		Warnings: env.stderr,
	}).Run(ctx, conn)
	if err != nil {
		return fail(err)
	}
	return exitOK
}
