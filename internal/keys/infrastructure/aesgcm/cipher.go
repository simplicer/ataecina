// Package aesgcm seals provider keys with AES-256-GCM under a random
// 32-byte master key, with "ataecina:v1:<name>" as associated data
// (SPECS §6).
package aesgcm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// MasterKeyBytes is the master key length.
const MasterKeyBytes = 32

// Cipher implements domain.Cipher.
type Cipher struct{}

// NewMasterKey draws 32 random bytes.
func (Cipher) NewMasterKey() (secret.Key, error) {
	raw := make([]byte, MasterKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return secret.Key{}, err
	}
	defer clear(raw)
	return secret.New(raw)
}

// Seal encrypts key under master, bound to its name.
func (Cipher) Seal(master secret.Key, name domain.KeyName, key secret.Key, setAt time.Time) (domain.SealedKey, error) {
	nonce, sealed, err := Seal(master, key.Reveal(), domain.AssociatedData(name))
	if err != nil {
		return domain.SealedKey{}, err
	}
	return domain.NewSealedKey(name, nonce, sealed, setAt), nil
}

// Open decrypts a sealed key; a wrong master key or a key moved to
// another name fails with domain.ErrBrokenSeal.
func (Cipher) Open(master secret.Key, sealed domain.SealedKey) (secret.Key, error) {
	plain, err := Open(master, sealed.Nonce(), sealed.Sealed(), domain.AssociatedData(sealed.Name()))
	if err != nil {
		return secret.Key{}, err
	}
	defer clear(plain)
	return secret.New(plain)
}

// Seal encrypts plain with a 256-bit key and a fresh 96-bit nonce.
func Seal(key secret.Key, plain, associated []byte) (nonce, sealed []byte, err error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plain, associated), nil
}

// Open decrypts what Seal produced.
func Open(key secret.Key, nonce, sealed, associated []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, domain.ErrBrokenSeal
	}
	plain, err := aead.Open(nil, nonce, sealed, associated)
	if err != nil {
		return nil, domain.ErrBrokenSeal
	}
	return plain, nil
}

func newAEAD(key secret.Key) (cipher.AEAD, error) {
	if key.Len() != MasterKeyBytes {
		return nil, fmt.Errorf("%w: the key must be %d bytes", domain.ErrBrokenSeal, MasterKeyBytes)
	}
	block, err := aes.NewCipher(key.Reveal())
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
