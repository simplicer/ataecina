package tray

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// The StatusNotifierItem the tray exports and the watcher it registers
// with (freedesktop StatusNotifierItem specification, as Plasma, the
// AppIndicator extension, XFCE, LXQt and Waybar implement it).
const (
	itemInterface    = "org.kde.StatusNotifierItem"
	itemPath         = dbus.ObjectPath("/StatusNotifierItem")
	watcherName      = "org.kde.StatusNotifierWatcher"
	watcherPath      = dbus.ObjectPath("/StatusNotifierWatcher")
	watcherInterface = "org.kde.StatusNotifierWatcher"
	iconName         = "ataecina"
)

var (
	errUnknownItem     = errors.New("no such menu item")
	errUnknownProperty = errors.New("no such menu item property")
)

// pixmap is one (iiay) icon image; the tray only names theme icons.
type pixmap struct {
	Width  int32
	Height int32
	Data   []byte
}

// toolTip is the (sa(iiay)ss) tooltip.
type toolTip struct {
	IconName    string
	IconPixmap  []pixmap
	Title       string
	Description string
}

// item answers the host's clicks. The host opens the menu itself
// (ItemIsMenu); a middle click refreshes.
type item struct {
	actions chan<- action
}

// Activate is a primary click; the menu opens instead.
func (item) Activate(int32, int32) *dbus.Error { return nil }

// SecondaryActivate is a middle click: refresh now.
func (i item) SecondaryActivate(int32, int32) *dbus.Error {
	select {
	case i.actions <- refreshAction:
	default:
	}
	return nil
}

// ContextMenu is handled by the host through the menu.
func (item) ContextMenu(int32, int32) *dbus.Error { return nil }

// Scroll does nothing.
func (item) Scroll(int32, string) *dbus.Error { return nil }

// export puts the item and its menu on the bus with their properties and
// introspection data, and returns the item's properties to update.
func export(conn *dbus.Conn, it item, m *menu) (*prop.Properties, error) {
	if err := conn.Export(it, itemPath, itemInterface); err != nil {
		return nil, fmt.Errorf("export the tray item: %w", err)
	}
	itemProperties, err := prop.Export(conn, itemPath, prop.Map{itemInterface: {
		"Category":            {Value: "ApplicationStatus"},
		"Id":                  {Value: "ataecina"},
		"Title":               {Value: "Ataecina"},
		"Status":              {Value: "Active"},
		"IconName":            {Value: iconName},
		"IconThemePath":       {Value: ""},
		"IconPixmap":          {Value: []pixmap{}},
		"OverlayIconName":     {Value: ""},
		"OverlayIconPixmap":   {Value: []pixmap{}},
		"AttentionIconName":   {Value: iconName},
		"AttentionIconPixmap": {Value: []pixmap{}},
		"AttentionMovieName":  {Value: ""},
		"ToolTip":             {Value: toolTip{IconName: iconName, IconPixmap: []pixmap{}, Title: "Ataecina"}},
		"ItemIsMenu":          {Value: true},
		"Menu":                {Value: menuPath},
	}})
	if err != nil {
		return nil, fmt.Errorf("export the tray item properties: %w", err)
	}
	statusArg := []introspect.Arg{{Name: "status", Type: "s"}}
	err = exportIntrospection(conn, itemPath, introspect.Interface{
		Name:       itemInterface,
		Methods:    introspect.Methods(it),
		Properties: itemProperties.Introspection(itemInterface),
		Signals: []introspect.Signal{
			{Name: "NewTitle"}, {Name: "NewIcon"}, {Name: "NewAttentionIcon"},
			{Name: "NewOverlayIcon"}, {Name: "NewToolTip"}, {Name: "NewStatus", Args: statusArg},
		},
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Export(m, menuPath, menuInterface); err != nil {
		return nil, fmt.Errorf("export the tray menu: %w", err)
	}
	menuProperties, err := prop.Export(conn, menuPath, prop.Map{menuInterface: {
		"Version":       {Value: uint32(3)},
		"TextDirection": {Value: "ltr"},
		"Status":        {Value: "normal"},
		"IconThemePath": {Value: []string{}},
	}})
	if err != nil {
		return nil, fmt.Errorf("export the tray menu properties: %w", err)
	}
	err = exportIntrospection(conn, menuPath, introspect.Interface{
		Name:       menuInterface,
		Methods:    introspect.Methods(m),
		Properties: menuProperties.Introspection(menuInterface),
		Signals: []introspect.Signal{
			{Name: "ItemsPropertiesUpdated", Args: []introspect.Arg{{Name: "updatedProps", Type: "a(ia{sv})"}, {Name: "removedProps", Type: "a(ias)"}}},
			{Name: "LayoutUpdated", Args: []introspect.Arg{{Name: "revision", Type: "u"}, {Name: "parent", Type: "i"}}},
			{Name: "ItemActivationRequested", Args: []introspect.Arg{{Name: "id", Type: "i"}, {Name: "timestamp", Type: "u"}}},
		},
	})
	if err != nil {
		return nil, err
	}
	m.layoutUpdated = func(revision uint32) {
		_ = conn.Emit(menuPath, menuInterface+".LayoutUpdated", revision, rootID)
	}
	return itemProperties, nil
}

func exportIntrospection(conn *dbus.Conn, path dbus.ObjectPath, own introspect.Interface) error {
	node := &introspect.Node{
		Name:       string(path),
		Interfaces: []introspect.Interface{introspect.IntrospectData, prop.IntrospectData, own},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("export introspection of %s: %w", path, err)
	}
	return nil
}
