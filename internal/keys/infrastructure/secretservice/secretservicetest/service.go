// Package secretservicetest is a fake org.freedesktop.secrets provider on
// a private bus, for tests only: it keeps items in memory and can pretend
// to be locked, show prompts and have them dismissed.
package secretservicetest

import (
	"fmt"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice"
)

const sessionPath = dbus.ObjectPath("/org/freedesktop/secrets/session/1")

type item struct {
	attributes map[string]string
	value      []byte
}

// Service is the fake Secret Service.
type Service struct {
	mu      sync.Mutex
	conn    *dbus.Conn
	items   map[dbus.ObjectPath]item
	next    int
	locked  bool
	dismiss bool
	prompts int
}

// Start claims org.freedesktop.secrets on the bus at address.
func Start(t testing.TB, address string) *Service {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if reply, err := conn.RequestName(secretservice.BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim %s: %v", secretservice.BusName, err)
	}
	s := &Service{conn: conn, items: map[dbus.ObjectPath]item{}}
	exports := []struct {
		object any
		path   dbus.ObjectPath
		iface  string
	}{
		{serviceObject{s}, secretservice.ServicePath, "org.freedesktop.Secret.Service"},
		{collectionObject{s}, secretservice.DefaultCollection, "org.freedesktop.Secret.Collection"},
		{sessionObject{}, sessionPath, "org.freedesktop.Secret.Session"},
	}
	for _, e := range exports {
		if err := conn.Export(e.object, e.path, e.iface); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// Lock makes the keyring locked until a prompt is answered.
func (s *Service) Lock() { s.mu.Lock(); s.locked = true; s.mu.Unlock() }

// DismissPrompts makes every prompt dismissed by the "person".
func (s *Service) DismissPrompts() { s.mu.Lock(); s.dismiss = true; s.mu.Unlock() }

// Prompts counts the prompts shown.
func (s *Service) Prompts() int { s.mu.Lock(); defer s.mu.Unlock(); return s.prompts }

// Items counts the stored items.
func (s *Service) Items() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.items) }

// Value returns the secret of the first item with the attributes.
func (s *Service) Value(attributes map[string]string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		if maps.Equal(it.attributes, attributes) {
			return it.value, true
		}
	}
	return nil, false
}

func locked(name string) *dbus.Error {
	return dbus.NewError("org.freedesktop.Secret.Error.IsLocked", []any{name + ": locked"})
}

type serviceObject struct{ s *Service }

func (o serviceObject) OpenSession(algorithm string, _ dbus.Variant) (dbus.Variant, dbus.ObjectPath, *dbus.Error) {
	if algorithm != "plain" {
		return dbus.Variant{}, "/", dbus.NewError("org.freedesktop.DBus.Error.NotSupported", []any{algorithm})
	}
	return dbus.MakeVariant(""), sessionPath, nil
}

func (o serviceObject) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, []dbus.ObjectPath, *dbus.Error) {
	o.s.mu.Lock()
	defer o.s.mu.Unlock()
	found := []dbus.ObjectPath{}
	for path, it := range o.s.items {
		matches := true
		for key, value := range attributes {
			matches = matches && it.attributes[key] == value
		}
		if matches {
			found = append(found, path)
		}
	}
	if o.s.locked {
		return []dbus.ObjectPath{}, found, nil
	}
	return found, []dbus.ObjectPath{}, nil
}

func (o serviceObject) Unlock(objects []dbus.ObjectPath) ([]dbus.ObjectPath, dbus.ObjectPath, *dbus.Error) {
	o.s.mu.Lock()
	defer o.s.mu.Unlock()
	if !o.s.locked {
		return objects, "/", nil
	}
	o.s.next++
	path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/prompt/%d", o.s.next))
	if err := o.s.conn.Export(promptObject{o.s, path}, path, "org.freedesktop.Secret.Prompt"); err != nil {
		return nil, "/", dbus.MakeFailedError(err)
	}
	return []dbus.ObjectPath{}, path, nil
}

func (o serviceObject) GetSecrets(items []dbus.ObjectPath, session dbus.ObjectPath) (map[dbus.ObjectPath]secretservice.Secret, *dbus.Error) {
	o.s.mu.Lock()
	defer o.s.mu.Unlock()
	if o.s.locked {
		return nil, locked("GetSecrets")
	}
	secrets := map[dbus.ObjectPath]secretservice.Secret{}
	for _, path := range items {
		if it, ok := o.s.items[path]; ok {
			secrets[path] = secretservice.Secret{Session: session, Parameters: []byte{}, Value: it.value, ContentType: "application/octet-stream"}
		}
	}
	return secrets, nil
}

type collectionObject struct{ s *Service }

func (o collectionObject) CreateItem(properties map[string]dbus.Variant, value secretservice.Secret, replace bool) (dbus.ObjectPath, dbus.ObjectPath, *dbus.Error) {
	o.s.mu.Lock()
	defer o.s.mu.Unlock()
	if o.s.locked {
		return "/", "/", locked("CreateItem")
	}
	attributes, _ := properties["org.freedesktop.Secret.Item.Attributes"].Value().(map[string]string)
	for path, it := range o.s.items {
		if replace && maps.Equal(it.attributes, attributes) {
			o.s.items[path] = item{attributes: attributes, value: value.Value}
			return path, "/", nil
		}
	}
	o.s.next++
	path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/login/%d", o.s.next))
	if err := o.s.conn.Export(itemObject{o.s, path}, path, "org.freedesktop.Secret.Item"); err != nil {
		return "/", "/", dbus.MakeFailedError(err)
	}
	o.s.items[path] = item{attributes: attributes, value: append([]byte(nil), value.Value...)}
	return path, "/", nil
}

type itemObject struct {
	s    *Service
	path dbus.ObjectPath
}

func (o itemObject) Delete() (dbus.ObjectPath, *dbus.Error) {
	o.s.mu.Lock()
	defer o.s.mu.Unlock()
	delete(o.s.items, o.path)
	return "/", nil
}

type sessionObject struct{}

func (sessionObject) Close() *dbus.Error { return nil }

type promptObject struct {
	s    *Service
	path dbus.ObjectPath
}

// Prompt "shows" the dialog and answers it shortly after.
func (o promptObject) Prompt(string) *dbus.Error {
	o.s.mu.Lock()
	o.s.prompts++
	dismissed := o.s.dismiss
	if !dismissed {
		o.s.locked = false
	}
	o.s.mu.Unlock()
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = o.s.conn.Emit(o.path, "org.freedesktop.Secret.Prompt.Completed", dismissed, dbus.MakeVariant([]dbus.ObjectPath{}))
	}()
	return nil
}
