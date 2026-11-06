// Package passphrase holds the master key wrapped with AES-256-GCM under
// a key derived from a passphrase by PBKDF2-HMAC-SHA256 (600 000
// iterations, 16-byte random salt). For SSH sessions, text consoles and
// machines without a desktop keyring (SPECS §6).
package passphrase

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

const (
	kdfName = "pbkdf2-sha256"
	// Iterations follows OWASP's current advice for PBKDF2-HMAC-SHA256.
	Iterations    = 600_000
	saltBytes     = 16
	minimumLength = 8
)

var associated = []byte("ataecina:v1:master-key:passphrase")

// ErrMismatch reports two different passphrases when confirming.
var ErrMismatch = errors.New("the passphrases do not match")

// Ask reads one passphrase; confirm asks for it twice when a person types.
type Ask func(prompt string, confirm bool) (secret.Key, error)

type record struct {
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Sealed     []byte `json:"sealed"`
}

// Unlocker implements domain.Unlocker with a passphrase.
type Unlocker struct {
	ask        Ask
	iterations int
}

// New asks for passphrases with ask. iterations is Iterations in
// production; tests may lower it.
func New(ask Ask, iterations int) Unlocker { return Unlocker{ask: ask, iterations: iterations} }

// Wrap asks for a new passphrase (twice on a terminal) and seals the
// master key under the key it derives.
func (u Unlocker) Wrap(_ context.Context, master secret.Key) (domain.WrappedMasterKey, error) {
	phrase, err := u.ask("New passphrase for the Ataecina master key: ", true)
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	defer phrase.Wipe()
	if phrase.Len() < minimumLength {
		return domain.WrappedMasterKey{}, fmt.Errorf("the passphrase needs at least %d characters", minimumLength)
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	derived, err := derive(phrase, salt, u.iterations)
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	defer derived.Wipe()
	nonce, sealed, err := aesgcm.Seal(derived, master.Reveal(), associated)
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	data, err := json.Marshal(record{KDF: kdfName, Iterations: u.iterations, Salt: salt, Nonce: nonce, Sealed: sealed})
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	return domain.NewWrappedMasterKey(domain.Passphrase, data), nil
}

// Unwrap asks for the passphrase and opens the master key; a wrong
// passphrase fails with domain.ErrBrokenSeal.
func (u Unlocker) Unwrap(_ context.Context, wrapped domain.WrappedMasterKey) (secret.Key, error) {
	var stored record
	if err := json.Unmarshal(wrapped.Data(), &stored); err != nil || stored.KDF != kdfName || stored.Iterations < 1 {
		return secret.Key{}, fmt.Errorf("the passphrase record is not one Ataecina can read")
	}
	phrase, err := u.ask("Passphrase for the Ataecina master key: ", false)
	if err != nil {
		return secret.Key{}, err
	}
	defer phrase.Wipe()
	derived, err := derive(phrase, stored.Salt, stored.Iterations)
	if err != nil {
		return secret.Key{}, err
	}
	defer derived.Wipe()
	plain, err := aesgcm.Open(derived, stored.Nonce, stored.Sealed, associated)
	if err != nil {
		return secret.Key{}, fmt.Errorf("wrong passphrase: %w", err)
	}
	defer clear(plain)
	return secret.New(plain)
}

// Forget has nothing to delete: the wrapped copy lives in keys.json.
func (Unlocker) Forget(context.Context, domain.WrappedMasterKey) error { return nil }

func derive(phrase secret.Key, salt []byte, iterations int) (secret.Key, error) {
	raw, err := pbkdf2.Key(sha256.New, string(phrase.Reveal()), salt, iterations, aesgcm.MasterKeyBytes)
	if err != nil {
		return secret.Key{}, err
	}
	defer clear(raw)
	return secret.New(raw)
}
