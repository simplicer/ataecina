package tray

import (
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"
)

// The com.canonical.dbusmenu object the item points at (version 3 of the
// protocol, as libdbusmenu, Plasma and the GNOME AppIndicator extension
// speak it).
const (
	menuInterface = "com.canonical.dbusmenu"
	menuPath      = dbus.ObjectPath("/MenuBar")
)

// Fixed ids of the entries that act; subscription lines follow from
// firstLineID, so a stale id from an old layout never triggers an action.
const (
	rootID int32 = iota
	refreshID
	quitID
	updatedID
	separatorID
	problemID
	firstLineID int32 = 100
)

type action int

const (
	noAction action = iota
	refreshAction
	quitAction
)

// entry is one item of the menu.
type entry struct {
	id        int32
	label     string
	enabled   bool
	separator bool
}

func (e entry) properties() map[string]dbus.Variant {
	if e.separator {
		return map[string]dbus.Variant{"type": dbus.MakeVariant("separator")}
	}
	return map[string]dbus.Variant{
		"label":   dbus.MakeVariant(e.label),
		"enabled": dbus.MakeVariant(e.enabled),
		"visible": dbus.MakeVariant(true),
	}
}

// layout is a node of GetLayout's (ia{sv}av) answer.
type layout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// itemProperties is one element of GetGroupProperties' a(ia{sv}) answer.
type itemProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}

// event is one element of EventGroup's a(isvu) argument.
type event struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

// menu holds the current entries and passes clicks to the tray's loop.
type menu struct {
	mu       sync.Mutex
	revision uint32
	entries  []entry
	actions  chan<- action
	// layoutUpdated announces a new revision; nil until exported.
	layoutUpdated func(revision uint32)
}

// show replaces the entries; it bumps the revision only when they change.
func (m *menu) show(entries []entry) {
	m.mu.Lock()
	if slices.Equal(entries, m.entries) {
		m.mu.Unlock()
		return
	}
	m.entries = entries
	m.revision++
	revision, announce := m.revision, m.layoutUpdated
	m.mu.Unlock()
	if announce != nil {
		announce(revision)
	}
}

func (m *menu) find(id int32) (entry, bool) {
	for _, e := range m.entries {
		if e.id == id {
			return e, true
		}
	}
	return entry{}, false
}

// GetLayout returns the whole menu: a root whose children are the entries.
func (m *menu) GetLayout(parentID int32, _ int32, _ []string) (uint32, layout, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if parentID != rootID {
		e, ok := m.find(parentID)
		if !ok {
			return 0, layout{}, dbus.MakeFailedError(errUnknownItem)
		}
		return m.revision, layout{ID: e.id, Properties: e.properties(), Children: []dbus.Variant{}}, nil
	}
	children := make([]dbus.Variant, 0, len(m.entries))
	for _, e := range m.entries {
		children = append(children, dbus.MakeVariant(layout{ID: e.id, Properties: e.properties(), Children: []dbus.Variant{}}))
	}
	root := map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}
	return m.revision, layout{ID: rootID, Properties: root, Children: children}, nil
}

// GetGroupProperties returns the properties of the asked entries.
func (m *menu) GetGroupProperties(ids []int32, _ []string) ([]itemProperties, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := make([]itemProperties, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.find(id); ok {
			found = append(found, itemProperties{ID: id, Properties: e.properties()})
		}
	}
	return found, nil
}

// GetProperty returns one property of one entry.
func (m *menu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.find(id)
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(errUnknownItem)
	}
	value, ok := e.properties()[name]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(errUnknownProperty)
	}
	return value, nil
}

// Event receives a click; only "clicked" on an acting entry does anything.
func (m *menu) Event(id int32, eventID string, _ dbus.Variant, _ uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	var chosen action
	switch id {
	case refreshID:
		chosen = refreshAction
	case quitID:
		chosen = quitAction
	default:
		return nil
	}
	select {
	case m.actions <- chosen:
	default: // the loop is busy with an earlier click; drop this one
	}
	return nil
}

// EventGroup receives several events at once.
func (m *menu) EventGroup(events []event) ([]int32, *dbus.Error) {
	for _, e := range events {
		_ = m.Event(e.ID, e.EventID, e.Data, e.Timestamp)
	}
	return []int32{}, nil
}

// AboutToShow says the menu needs no update before it opens.
func (m *menu) AboutToShow(int32) (bool, *dbus.Error) { return false, nil }

// AboutToShowGroup says no entry needs an update before it opens.
func (m *menu) AboutToShowGroup([]int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}
