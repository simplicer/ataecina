// Package systemdtest is a fake systemd user manager on a private bus that
// records the units it is asked to start. For tests only.
package systemdtest

import (
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/platform/systemdunit"
)

// Manager records StartUnit calls.
type Manager struct {
	mu      sync.Mutex
	started []string
}

// Start claims org.freedesktop.systemd1 on the bus at address.
func Start(t testing.TB, address string) *Manager {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if reply, err := conn.RequestName(systemdunit.BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim %s: %v", systemdunit.BusName, err)
	}
	m := &Manager{}
	if err := conn.Export(m, systemdunit.ObjectPath, "org.freedesktop.systemd1.Manager"); err != nil {
		t.Fatal(err)
	}
	return m
}

// StartUnit implements the D-Bus method.
func (m *Manager) StartUnit(name, mode string) (dbus.ObjectPath, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, name+" "+mode)
	return dbus.ObjectPath("/org/freedesktop/systemd1/job/1"), nil
}

// Started lists "unit mode" for every start asked.
func (m *Manager) Started() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.started...)
}
