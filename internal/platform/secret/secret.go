// Package secret holds a credential in memory without ever printing it:
// String, GoString, Format and MarshalJSON all redact it (AGENTS.md,
// security rules). It is the one value type both the keys and the
// allowances contexts use for a provider key.
package secret

import (
	"errors"
	"fmt"
)

const redacted = "[REDACTED]"

// ErrEmpty reports an empty secret.
var ErrEmpty = errors.New("empty secret")

// Key is a secret value: a provider API key or a master key.
type Key struct {
	value []byte
}

// New copies value into a Key.
func New(value []byte) (Key, error) {
	if len(value) == 0 {
		return Key{}, ErrEmpty
	}
	return Key{value: append([]byte(nil), value...)}, nil
}

// Reveal gives the raw bytes to the one place that must send them.
func (k Key) Reveal() []byte { return k.value }

// Len is the length in bytes.
func (k Key) Len() int { return len(k.value) }

// Wipe overwrites the bytes in place (best effort: Go may keep copies).
func (k Key) Wipe() {
	for i := range k.value {
		k.value[i] = 0
	}
}

func (Key) String() string   { return redacted }
func (Key) GoString() string { return redacted }

// Format redacts every verb, including %x and %#v.
func (Key) Format(state fmt.State, _ rune) { _, _ = state.Write([]byte(redacted)) }

// MarshalJSON redacts the key in any JSON.
func (Key) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText redacts the key in text encodings.
func (Key) MarshalText() ([]byte, error) { return []byte(redacted), nil }
