//go:build integration

package secretservice_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/secretservice/secretservicetest"
	"codeberg.org/simplicer/ataecina/internal/platform/dbustest"
)

// Every test runs against a private dbus-daemon and a fake Secret
// Service; the owner's keyring is never touched.

func connector(address string) func() (*dbus.Conn, error) {
	return func() (*dbus.Conn, error) { return dbus.Connect(address) }
}

func context2s(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestTheMasterKeyLivesInTheKeyring(t *testing.T) {
	address := dbustest.Start(t)
	fake := secretservicetest.Start(t, address)
	unlocker := secretservice.New(connector(address), true)
	if !unlocker.Reachable() {
		t.Fatal("the fake service must be reachable")
	}
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	wrapped, err := unlocker.Wrap(context2s(t), master)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped.Kind() != domain.SecretService || bytes.Contains(wrapped.Data(), master.Reveal()) {
		t.Fatalf("record = %s", wrapped.Data())
	}
	if stored, ok := fake.Value(secretservice.Attributes); !ok || !bytes.Equal(stored, master.Reveal()) {
		t.Fatal("the keyring must hold the master key under the Ataecina attributes")
	}
	if _, err := unlocker.Wrap(context2s(t), master); err != nil || fake.Items() != 1 {
		t.Fatalf("wrapping again replaces the item: %v, %d items", err, fake.Items())
	}
	opened, err := unlocker.Unwrap(context2s(t), wrapped)
	if err != nil || !bytes.Equal(opened.Reveal(), master.Reveal()) {
		t.Fatalf("Unwrap = %v", err)
	}
	if err := unlocker.Forget(context2s(t), wrapped); err != nil || fake.Items() != 0 {
		t.Fatalf("Forget = %v, %d items", err, fake.Items())
	}
	if _, err := unlocker.Unwrap(context2s(t), wrapped); err == nil {
		t.Fatal("a forgotten master key cannot be read")
	}
}

func TestALockedKeyringPromptsOnlyWhenAPersonIsThere(t *testing.T) {
	address := dbustest.Start(t)
	fake := secretservicetest.Start(t, address)
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	wrapped, err := secretservice.New(connector(address), true).Wrap(context2s(t), master)
	if err != nil {
		t.Fatal(err)
	}
	fake.Lock()
	if _, err := secretservice.New(connector(address), false).Unwrap(context2s(t), wrapped); !errors.Is(err, domain.ErrUnlockerUnavailable) || fake.Prompts() != 0 {
		t.Fatalf("the collector never prompts: %v, %d prompts", err, fake.Prompts())
	}
	opened, err := secretservice.New(connector(address), true).Unwrap(context2s(t), wrapped)
	if err != nil || !bytes.Equal(opened.Reveal(), master.Reveal()) || fake.Prompts() != 1 {
		t.Fatalf("interactive unlock = %v, %d prompts", err, fake.Prompts())
	}
	fake.Lock()
	fake.DismissPrompts()
	if _, err := secretservice.New(connector(address), true).Unwrap(context2s(t), wrapped); err == nil {
		t.Fatal("a dismissed prompt must fail")
	}
	if _, err := secretservice.New(connector(address), true).Wrap(context2s(t), master); err == nil {
		t.Fatal("a dismissed prompt must fail when storing too")
	}
}

func TestWithoutASecretServiceTheUnlockerIsUnavailable(t *testing.T) {
	address := dbustest.Start(t)
	empty := secretservice.New(connector(address), true)
	if empty.Reachable() {
		t.Fatal("a bus without the service is not reachable")
	}
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	if _, err := empty.Wrap(context2s(t), master); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("Wrap = %v", err)
	}
	broken := secretservice.New(func() (*dbus.Conn, error) { return nil, errors.New("no bus") }, true)
	if broken.Reachable() {
		t.Fatal("no bus is not reachable")
	}
	wrapped := domain.NewWrappedMasterKey(domain.SecretService, []byte(`{}`))
	if _, err := broken.Unwrap(context2s(t), wrapped); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("Unwrap = %v", err)
	}
	if err := broken.Forget(context2s(t), wrapped); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("Forget = %v", err)
	}
}
