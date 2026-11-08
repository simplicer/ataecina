// Package traytest plays the panel's side of the tray on a private bus: a
// fake StatusNotifierWatcher that records registrations, and a client that
// reads the item's properties and reads and clicks its menu, speaking the
// protocol independently of the tray's own code. For tests only.
package traytest

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	watcherName   = "org.kde.StatusNotifierWatcher"
	itemPath      = dbus.ObjectPath("/StatusNotifierItem")
	itemInterface = "org.kde.StatusNotifierItem"
	menuInterface = "com.canonical.dbusmenu"
)

// Watcher is a fake org.kde.StatusNotifierWatcher.
type Watcher struct {
	registered chan string
}

// StartWatcher claims the watcher's name on the bus at address.
func StartWatcher(t testing.TB, address string) *Watcher {
	t.Helper()
	conn := connect(t, address)
	// Like a real watcher, the object exists before the name is claimed:
	// a tray that sees the name can register at once.
	w := &Watcher{registered: make(chan string, 8)}
	if err := conn.Export(w, "/StatusNotifierWatcher", watcherName); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(watcherName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim %s: %v", watcherName, err)
	}
	return w
}

// RegisterStatusNotifierItem implements the D-Bus method.
func (w *Watcher) RegisterStatusNotifierItem(service string) *dbus.Error {
	w.registered <- service
	return nil
}

// Registered waits for an item to register and returns its bus name.
func (w *Watcher) Registered(t testing.TB) string {
	t.Helper()
	select {
	case service := <-w.registered:
		return service
	case <-time.After(10 * time.Second):
		t.Fatal("no item registered with the watcher")
		return ""
	}
}

// Panel reads and clicks one registered item.
type Panel struct {
	t       testing.TB
	conn    *dbus.Conn
	service string
}

// NewPanel connects to the bus at address to talk to service.
func NewPanel(t testing.TB, address, service string) *Panel {
	t.Helper()
	return &Panel{t: t, conn: connect(t, address), service: service}
}

// Property reads one property of the item.
func (p *Panel) Property(name string) dbus.Variant {
	p.t.Helper()
	value, err := p.conn.Object(p.service, itemPath).GetProperty(itemInterface + "." + name)
	if err != nil {
		p.t.Fatalf("item property %s: %v", name, err)
	}
	return value
}

// node is one (ia{sv}av) layout node.
type node struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// Entry is one menu entry as a panel shows it.
type Entry struct {
	ID        int32
	Label     string
	Enabled   bool
	Separator bool
}

// Menu reads the whole menu.
func (p *Panel) Menu() []Entry {
	p.t.Helper()
	var revision uint32
	var root node
	err := p.menuObject().Call(menuInterface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&revision, &root)
	if err != nil {
		p.t.Fatalf("GetLayout: %v", err)
	}
	entries := make([]Entry, 0, len(root.Children))
	for _, child := range root.Children {
		var n node
		if err := dbus.Store([]any{child.Value()}, &n); err != nil {
			p.t.Fatalf("menu entry: %v", err)
		}
		e := Entry{ID: n.ID}
		if kind, _ := n.Properties["type"].Value().(string); kind == "separator" {
			e.Separator = true
		}
		e.Label, _ = n.Properties["label"].Value().(string)
		e.Enabled, _ = n.Properties["enabled"].Value().(bool)
		entries = append(entries, e)
	}
	return entries
}

// Click sends "clicked" to the entry with exactly this label.
func (p *Panel) Click(label string) {
	p.t.Helper()
	for _, e := range p.Menu() {
		if e.Label == label {
			err := p.menuObject().Call(menuInterface+".Event", 0, e.ID, "clicked", dbus.MakeVariant(""), uint32(0)).Err
			if err != nil {
				p.t.Fatalf("click %q: %v", label, err)
			}
			return
		}
	}
	p.t.Fatalf("no menu entry %q", label)
}

// MiddleClick is the panel's secondary activation of the item.
func (p *Panel) MiddleClick() {
	p.t.Helper()
	if err := p.conn.Object(p.service, itemPath).Call(itemInterface+".SecondaryActivate", 0, int32(0), int32(0)).Err; err != nil {
		p.t.Fatalf("SecondaryActivate: %v", err)
	}
}

func (p *Panel) menuObject() dbus.BusObject {
	p.t.Helper()
	path, ok := p.Property("Menu").Value().(dbus.ObjectPath)
	if !ok {
		p.t.Fatal("the item names no menu")
	}
	return p.conn.Object(p.service, path)
}

func connect(t testing.TB, address string) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
