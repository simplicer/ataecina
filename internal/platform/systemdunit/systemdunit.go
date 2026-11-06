// Package systemdunit asks the user's systemd manager, over the session
// bus, to start a unit — what `systemctl --user start` does, without
// running a program. The tray uses it for "Refresh now".
package systemdunit

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

// D-Bus names of the systemd manager.
const (
	BusName    = "org.freedesktop.systemd1"
	ObjectPath = dbus.ObjectPath("/org/freedesktop/systemd1")
	startUnit  = "org.freedesktop.systemd1.Manager.StartUnit"
)

// Start queues a start job for unit, replacing a pending one, and returns
// once systemd has accepted it (not when the unit finishes).
func Start(ctx context.Context, conn *dbus.Conn, unit string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var job dbus.ObjectPath
	err := conn.Object(BusName, ObjectPath).CallWithContext(ctx, startUnit, 0, unit, "replace").Store(&job)
	if err != nil {
		return fmt.Errorf("start %s: %w", unit, err)
	}
	return nil
}
