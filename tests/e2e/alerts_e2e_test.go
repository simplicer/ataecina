//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify/notifytest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

func TestTheCollectorNotifiesTheDesktop(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "claude", "source": "e2e-fake"}]}`, map[string]string{
		"claude": `{"meters": [{"window_minutes": 300, "used_percent": 96, "resets_in_minutes": 40}]}`,
	})
	address := dbustest.Start(t)
	desktop := notifytest.Start(t, address)
	tr.setenv("DBUS_SESSION_BUS_ADDRESS", address)
	for range 2 {
		if code, _, stderr := tr.run(bin, "collect"); code != 0 {
			t.Fatalf("collect = %d %s", code, stderr)
		}
	}
	shown := desktop.Shown()
	if len(shown) != 1 || shown[0].Summary != "claude · five hours at 96 %" || !strings.Contains(shown[0].Body, "Reborn in") || shown[0].Icon != "ataecina" {
		t.Fatalf("shown = %+v", shown)
	}
}
