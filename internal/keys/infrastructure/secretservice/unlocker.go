// Package secretservice holds the master key as an item of the session's
// org.freedesktop.secrets provider (GNOME Keyring, KWallet/ksecretd,
// KeePassXC), with the attributes {application: ataecina, purpose:
// master-key} (SPECS §6). The secret travels over the per-user session
// bus with the "plain" algorithm.
package secretservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// D-Bus names of the Secret Service API.
const (
	BusName           = "org.freedesktop.secrets"
	ServicePath       = dbus.ObjectPath("/org/freedesktop/secrets")
	DefaultCollection = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
	serviceInterface  = "org.freedesktop.Secret.Service"
	collectionIface   = "org.freedesktop.Secret.Collection"
	itemInterface     = "org.freedesktop.Secret.Item"
	sessionInterface  = "org.freedesktop.Secret.Session"
	promptInterface   = "org.freedesktop.Secret.Prompt"
	noPrompt          = dbus.ObjectPath("/")
	itemLabel         = "Ataecina master key"
	contentType       = "application/octet-stream"
	masterKeyBytes    = 32
)

// Attributes find the Ataecina master key in the keyring.
var Attributes = map[string]string{"application": "ataecina", "purpose": "master-key"}

// Secret is the Secret Service (oayays) struct.
type Secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

type record struct {
	Attributes map[string]string `json:"attributes"`
}

// Unlocker implements domain.Unlocker with the Secret Service.
type Unlocker struct {
	connect     func() (*dbus.Conn, error)
	interactive bool
}

// New connects with connect for every operation. When interactive is
// false (the collector) a locked keyring is "unavailable" instead of
// showing the keyring's unlock prompt.
func New(connect func() (*dbus.Conn, error), interactive bool) Unlocker {
	return Unlocker{connect: connect, interactive: interactive}
}

// Reachable reports whether some process provides the Secret Service.
func (u Unlocker) Reachable() bool {
	conn, err := u.connect()
	if err != nil {
		return false
	}
	defer conn.Close()
	var owned bool
	if conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, BusName).Store(&owned) == nil && owned {
		return true
	}
	var activatable []string
	if conn.BusObject().Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&activatable) != nil {
		return false
	}
	for _, name := range activatable {
		if name == BusName {
			return true
		}
	}
	return false
}

// Owner tells which process provides the Secret Service: its unique bus
// name and process id (for `ataecina doctor`).
func (u Unlocker) Owner() (string, uint32, error) {
	conn, err := u.connect()
	if err != nil {
		return "", 0, err
	}
	defer conn.Close()
	var name string
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, BusName).Store(&name); err != nil {
		return "", 0, err
	}
	var pid uint32
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid); err != nil {
		return name, 0, err
	}
	return name, pid, nil
}

// Wrap stores the master key as a keyring item, replacing an older one.
func (u Unlocker) Wrap(ctx context.Context, master secret.Key) (domain.WrappedMasterKey, error) {
	s, err := u.open()
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	defer s.close()
	if err := s.unlock(ctx, []dbus.ObjectPath{DefaultCollection}); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	properties := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label":      dbus.MakeVariant(itemLabel),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(Attributes),
	}
	value := Secret{Session: s.session, Parameters: []byte{}, Value: master.Reveal(), ContentType: contentType}
	var item, prompt dbus.ObjectPath
	if err := s.conn.Object(BusName, DefaultCollection).CallWithContext(ctx, collectionIface+".CreateItem", 0, properties, value, true).Store(&item, &prompt); err != nil {
		return domain.WrappedMasterKey{}, fmt.Errorf("create the keyring item: %w", err)
	}
	if err := s.await(ctx, prompt); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	data, err := json.Marshal(record{Attributes: Attributes})
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	return domain.NewWrappedMasterKey(domain.SecretService, data), nil
}

// Unwrap reads the master key from the keyring.
func (u Unlocker) Unwrap(ctx context.Context, wrapped domain.WrappedMasterKey) (secret.Key, error) {
	s, err := u.open()
	if err != nil {
		return secret.Key{}, err
	}
	defer s.close()
	items, err := s.items(ctx, attributesOf(wrapped))
	if err != nil {
		return secret.Key{}, err
	}
	if len(items) == 0 {
		return secret.Key{}, errors.New("the keyring has no Ataecina master key item")
	}
	var secrets map[dbus.ObjectPath]Secret
	if err := s.service().CallWithContext(ctx, serviceInterface+".GetSecrets", 0, items[:1], s.session).Store(&secrets); err != nil {
		return secret.Key{}, fmt.Errorf("read the keyring item: %w", err)
	}
	found, ok := secrets[items[0]]
	if !ok || len(found.Value) != masterKeyBytes {
		return secret.Key{}, fmt.Errorf("%w: the keyring item is not a master key", domain.ErrBrokenSeal)
	}
	defer clear(found.Value)
	return secret.New(found.Value)
}

// Forget deletes the keyring items that hold the master key.
func (u Unlocker) Forget(ctx context.Context, wrapped domain.WrappedMasterKey) error {
	s, err := u.open()
	if err != nil {
		return err
	}
	defer s.close()
	items, err := s.items(ctx, attributesOf(wrapped))
	if err != nil {
		return err
	}
	for _, item := range items {
		var prompt dbus.ObjectPath
		if err := s.conn.Object(BusName, item).CallWithContext(ctx, itemInterface+".Delete", 0).Store(&prompt); err != nil {
			return fmt.Errorf("delete the keyring item: %w", err)
		}
		if err := s.await(ctx, prompt); err != nil {
			return err
		}
	}
	return nil
}

func attributesOf(wrapped domain.WrappedMasterKey) map[string]string {
	var stored record
	if json.Unmarshal(wrapped.Data(), &stored) != nil || len(stored.Attributes) == 0 {
		return Attributes
	}
	return stored.Attributes
}

// session is one connection with an open Secret Service session.
type session struct {
	conn        *dbus.Conn
	session     dbus.ObjectPath
	interactive bool
}

func (u Unlocker) open() (*session, error) {
	conn, err := u.connect()
	if err != nil {
		return nil, fmt.Errorf("%w: no session bus: %w", domain.ErrUnlockerUnavailable, err)
	}
	var output dbus.Variant
	var path dbus.ObjectPath
	if err := conn.Object(BusName, ServicePath).Call(serviceInterface+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &path); err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: no Secret Service: %w", domain.ErrUnlockerUnavailable, err)
	}
	return &session{conn: conn, session: path, interactive: u.interactive}, nil
}

func (s *session) close() {
	_ = s.conn.Object(BusName, s.session).Call(sessionInterface+".Close", 0).Err
	_ = s.conn.Close()
}

func (s *session) service() dbus.BusObject { return s.conn.Object(BusName, ServicePath) }

// items finds the items with the attributes, unlocking them if needed.
func (s *session) items(ctx context.Context, attributes map[string]string) ([]dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	if err := s.service().CallWithContext(ctx, serviceInterface+".SearchItems", 0, attributes).Store(&unlocked, &locked); err != nil {
		return nil, fmt.Errorf("search the keyring: %w", err)
	}
	if len(unlocked) > 0 || len(locked) == 0 {
		return unlocked, nil
	}
	if err := s.unlock(ctx, locked); err != nil {
		return nil, err
	}
	return locked, nil
}

func (s *session) unlock(ctx context.Context, objects []dbus.ObjectPath) error {
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := s.service().CallWithContext(ctx, serviceInterface+".Unlock", 0, objects).Store(&unlocked, &prompt); err != nil {
		return fmt.Errorf("unlock the keyring: %w", err)
	}
	if prompt == noPrompt {
		return nil
	}
	if !s.interactive {
		return fmt.Errorf("%w: the keyring is locked", domain.ErrUnlockerUnavailable)
	}
	return s.prompt(ctx, prompt)
}

// await answers a prompt the service asked for; "/" means none.
func (s *session) await(ctx context.Context, prompt dbus.ObjectPath) error {
	if prompt == noPrompt {
		return nil
	}
	return s.prompt(ctx, prompt)
}

// prompt shows the keyring's own dialog and waits for its answer.
func (s *session) prompt(ctx context.Context, prompt dbus.ObjectPath) error {
	if err := s.conn.AddMatchSignal(dbus.WithMatchObjectPath(prompt), dbus.WithMatchInterface(promptInterface), dbus.WithMatchMember("Completed")); err != nil {
		return err
	}
	signals := make(chan *dbus.Signal, 4)
	s.conn.Signal(signals)
	defer s.conn.RemoveSignal(signals)
	if err := s.conn.Object(BusName, prompt).CallWithContext(ctx, promptInterface+".Prompt", 0, "").Err; err != nil {
		return fmt.Errorf("show the keyring prompt: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: no answer from the keyring prompt", domain.ErrUnlockerUnavailable)
		case signal := <-signals:
			if signal.Path != prompt || signal.Name != promptInterface+".Completed" || len(signal.Body) == 0 {
				continue
			}
			if dismissed, _ := signal.Body[0].(bool); dismissed {
				return errors.New("the keyring prompt was dismissed")
			}
			return nil
		}
	}
}
