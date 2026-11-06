//go:build e2e

package e2e

import (
	"bytes"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/tray/traytest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit/systemdtest"
)

// startTray runs the production binary's tray against a private bus.
func (tr *tree) startTray(bin string) (*exec.Cmd, *bytes.Buffer) {
	tr.t.Helper()
	cmd := exec.Command(bin, "tray")
	cmd.Env = tr.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		tr.t.Fatal(err)
	}
	tr.t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd, &stderr
}

func waitExit(t *testing.T, cmd *exec.Cmd, stderr *bytes.Buffer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("tray exited with %v: %s", err, stderr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tray did not exit")
	}
}

func TestTheTrayShowsTheCollectedSnapshot(t *testing.T) {
	collector := binary(t, "ATAECINA_E2E_BINARY")
	production := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, config, fakes)
	if code, _, stderr := tr.run(collector, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}

	address := dbustest.Start(t)
	watcher := traytest.StartWatcher(t, address)
	manager := systemdtest.Start(t, address)
	tr.setenv("DBUS_SESSION_BUS_ADDRESS", address)
	cmd, stderr := tr.startTray(production)

	panel := traytest.NewPanel(t, address, watcher.Registered(t))
	if got := panel.Property("IconName").Value(); got != "ataecina" {
		t.Fatalf("IconName = %v", got)
	}
	var labels []string
	for _, e := range panel.Menu() {
		labels = append(labels, e.Label)
	}
	menu := strings.Join(labels, "\n")
	for _, fragment := range []string{"claude · PRO · 5h 23.5% ↻ 2h10m", "minimax · broken", "openrouter · $8.42 left", "zai · key missing", "Refresh now", "Quit"} {
		if !strings.Contains(menu, fragment) {
			t.Fatalf("the menu lacks %q:\n%s", fragment, menu)
		}
	}

	panel.MiddleClick()
	deadline := time.Now().Add(5 * time.Second)
	for !slices.Equal(manager.Started(), []string{"ataecina-collect.service replace"}) {
		if time.Now().After(deadline) {
			t.Fatalf("started = %v", manager.Started())
		}
		time.Sleep(20 * time.Millisecond)
	}
	panel.Click("Quit")
	waitExit(t, cmd, stderr)
}

func TestTheTrayStopsOnSIGTERM(t *testing.T) {
	production := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, config, fakes)
	address := dbustest.Start(t)
	watcher := traytest.StartWatcher(t, address)
	tr.setenv("DBUS_SESSION_BUS_ADDRESS", address)
	cmd, stderr := tr.startTray(production)
	panel := traytest.NewPanel(t, address, watcher.Registered(t))
	if labels := panel.Menu(); !strings.HasPrefix(labels[0].Label, "No snapshot yet") {
		t.Fatalf("menu before any collection = %+v", labels)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitExit(t, cmd, stderr)
}
