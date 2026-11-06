package aesgcm_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

func TestASealedKeyOpensOnlyWithItsMasterKeyAndName(t *testing.T) {
	c := aesgcm.Cipher{}
	master, err := c.NewMasterKey()
	if err != nil || master.Len() != 32 {
		t.Fatalf("NewMasterKey = %v", err)
	}
	other, _ := c.NewMasterKey()
	if bytes.Equal(master.Reveal(), other.Reveal()) {
		t.Fatal("master keys must be random")
	}
	provider, _ := secret.New([]byte("sk-zai-synthetic"))
	sealed, err := c.Seal(master, "zai", provider, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed.Sealed(), []byte("sk-zai")) || len(sealed.Nonce()) != 12 {
		t.Fatal("the sealed value must not contain the key")
	}
	opened, err := c.Open(master, sealed)
	if err != nil || string(opened.Reveal()) != "sk-zai-synthetic" {
		t.Fatalf("Open = %v", err)
	}
	if _, err := c.Open(other, sealed); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("another master key = %v", err)
	}
	moved := domain.NewSealedKey("openrouter", sealed.Nonce(), sealed.Sealed(), sealed.SetAt())
	if _, err := c.Open(master, moved); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("a key moved to another provider = %v", err)
	}
	truncated := domain.NewSealedKey("zai", sealed.Nonce()[:4], sealed.Sealed(), sealed.SetAt())
	if _, err := c.Open(master, truncated); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("a bad nonce = %v", err)
	}
	short, _ := secret.New([]byte("short"))
	if _, err := c.Seal(short, "zai", provider, time.Now()); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("a short master key = %v", err)
	}
	if _, _, err := aesgcm.Seal(short, nil, nil); err == nil {
		t.Fatal("Seal with a short key must fail")
	}
	if _, err := aesgcm.Open(short, nil, nil, nil); err == nil {
		t.Fatal("Open with a short key must fail")
	}
}
