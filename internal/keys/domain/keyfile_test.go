package domain_test

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
)

var setAt = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func TestKeyNamesAndUnlockerKinds(t *testing.T) {
	if name, err := domain.NewKeyName("zai-coding"); err != nil || name.String() != "zai-coding" {
		t.Fatalf("NewKeyName = %q, %v", name, err)
	}
	for _, bad := range []string{"", "Zai", "../x", "a b"} {
		if _, err := domain.NewKeyName(bad); !errors.Is(err, domain.ErrInvalidKeyName) {
			t.Fatalf("NewKeyName(%q) = %v", bad, err)
		}
	}
	for _, kind := range []string{"secret-service", "systemd-creds", "passphrase"} {
		if parsed, err := domain.ParseUnlockerKind(kind); err != nil || string(parsed) != kind {
			t.Fatalf("ParseUnlockerKind(%s) = %v", kind, err)
		}
	}
	if _, err := domain.ParseUnlockerKind("kwallet"); err == nil {
		t.Fatal("unknown unlockers are refused")
	}
	if string(domain.AssociatedData("zai")) != "ataecina:v1:zai" {
		t.Fatal("associated data must bind the provider name")
	}
}

func TestAKeyFileAlwaysKeepsOneUnlocker(t *testing.T) {
	file := domain.NewKeyFile(domain.NewWrappedMasterKey(domain.Passphrase, []byte("p")))
	if err := file.AddUnlocker(domain.NewWrappedMasterKey(domain.SecretService, nil)); err != nil {
		t.Fatal(err)
	}
	if err := file.AddUnlocker(domain.NewWrappedMasterKey(domain.SecretService, nil)); !errors.Is(err, domain.ErrUnlockerExists) {
		t.Fatalf("duplicate unlocker = %v", err)
	}
	if got := file.Unlockers(); len(got) != 2 || got[0].Kind() != domain.SecretService || string(got[1].Data()) != "p" {
		t.Fatalf("unlockers in order = %+v", got)
	}
	if _, err := file.RemoveUnlocker(domain.SystemdCreds); !errors.Is(err, domain.ErrUnlockerMissing) {
		t.Fatalf("missing unlocker = %v", err)
	}
	if removed, err := file.RemoveUnlocker(domain.SecretService); err != nil || removed.Kind() != domain.SecretService {
		t.Fatalf("RemoveUnlocker = %v", err)
	}
	if _, err := file.RemoveUnlocker(domain.Passphrase); !errors.Is(err, domain.ErrLastUnlocker) {
		t.Fatalf("last unlocker = %v", err)
	}
}

func TestSealedKeysAreKeptByName(t *testing.T) {
	file, err := domain.RestoreKeyFile(
		[]domain.WrappedMasterKey{domain.NewWrappedMasterKey(domain.Passphrase, nil)},
		[]domain.SealedKey{domain.NewSealedKey("zai", []byte{1}, []byte{2}, setAt)},
	)
	if err != nil {
		t.Fatal(err)
	}
	file.PutKey(domain.NewSealedKey("openrouter", []byte{3}, []byte{4}, setAt))
	keys := file.Keys()
	if len(keys) != 2 || keys[0].Name() != "openrouter" || keys[1].Nonce()[0] != 1 || keys[1].Sealed()[0] != 2 || !keys[1].SetAt().Equal(setAt) {
		t.Fatalf("keys = %+v", keys)
	}
	if _, ok := file.Key("zai"); !ok {
		t.Fatal("Key(zai) not found")
	}
	if err := file.ForgetKey("zai"); err != nil {
		t.Fatal(err)
	}
	if err := file.ForgetKey("zai"); !errors.Is(err, domain.ErrKeyNotFound) {
		t.Fatalf("forgotten twice = %v", err)
	}
	if _, err := domain.RestoreKeyFile(nil, nil); err == nil {
		t.Fatal("a key file needs an unlocker")
	}
}
