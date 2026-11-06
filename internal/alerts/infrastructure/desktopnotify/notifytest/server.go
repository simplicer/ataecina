// Package notifytest is a fake org.freedesktop.Notifications server on a
// private bus that records what it is asked to show. For tests only.
package notifytest

import (
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify"
)

// Shown is one notification received.
type Shown struct {
	App, Icon, Summary, Body string
	Urgency                  byte
}

// Server records notifications.
type Server struct {
	mu    sync.Mutex
	shown []Shown
}

// Start claims org.freedesktop.Notifications on the bus at address.
func Start(t testing.TB, address string) *Server {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if reply, err := conn.RequestName(desktopnotify.BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim %s: %v", desktopnotify.BusName, err)
	}
	s := &Server{}
	if err := conn.Export(s, desktopnotify.ObjectPath, "org.freedesktop.Notifications"); err != nil {
		t.Fatal(err)
	}
	return s
}

// Notify implements the D-Bus method.
func (s *Server) Notify(app string, _ uint32, icon, summary, body string, _ []string, hints map[string]dbus.Variant, _ int32) (uint32, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	urgency, _ := hints["urgency"].Value().(byte)
	s.shown = append(s.shown, Shown{App: app, Icon: icon, Summary: summary, Body: body, Urgency: urgency})
	return uint32(len(s.shown)), nil
}

// Shown lists what was shown.
func (s *Server) Shown() []Shown {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Shown(nil), s.shown...)
}
