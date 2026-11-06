// Package domain holds the keys model: sealed provider keys, the master
// key that seals them and the unlockers that hold it (SPECS §6, ADR 0004).
// It knows nothing of quotas, files, D-Bus or terminals.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// Errors of the keys context.
var (
	ErrNotInitialized      = errors.New("keys are not initialised: run `ataecina init`")
	ErrAlreadyInitialized  = errors.New("keys are already initialised")
	ErrKeyNotFound         = errors.New("no sealed key with that name")
	ErrLocked              = errors.New("no unlocker could open the master key")
	ErrUnlockerExists      = errors.New("that unlocker already holds the master key")
	ErrUnlockerMissing     = errors.New("that unlocker does not hold the master key")
	ErrLastUnlocker        = errors.New("the last unlocker cannot be removed")
	ErrUnlockerUnavailable = errors.New("that unlocker cannot work here")
	ErrInvalidKeyName      = errors.New("invalid key name")
	ErrBrokenSeal          = errors.New("a sealed value cannot be opened with this master key")
)

var keyNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// KeyName names a sealed provider key (by default the subscription id).
type KeyName string

// NewKeyName validates a key name.
func NewKeyName(raw string) (KeyName, error) {
	if !keyNamePattern.MatchString(raw) {
		return "", fmt.Errorf("%w: %q (lowercase letters, digits and dashes, up to 32)", ErrInvalidKeyName, raw)
	}
	return KeyName(raw), nil
}

func (n KeyName) String() string { return string(n) }

// AssociatedData binds a sealed key to its name, so a sealed value cannot
// be moved to another provider.
func AssociatedData(name KeyName) []byte { return []byte("ataecina:v1:" + string(name)) }

// UnlockerKind is a way to hold the master key.
type UnlockerKind string

// Unlocker kinds.
const (
	SecretService UnlockerKind = "secret-service"
	SystemdCreds  UnlockerKind = "systemd-creds"
	Passphrase    UnlockerKind = "passphrase"
)

// UnlockOrder tries the unlockers that need no person first.
var UnlockOrder = []UnlockerKind{SecretService, SystemdCreds, Passphrase}

// ParseUnlockerKind validates an unlocker kind.
func ParseUnlockerKind(raw string) (UnlockerKind, error) {
	for _, kind := range UnlockOrder {
		if string(kind) == raw {
			return kind, nil
		}
	}
	return "", fmt.Errorf("unknown unlocker %q (secret-service, systemd-creds or passphrase)", raw)
}

// WrappedMasterKey is how one unlocker holds the master key; its data is
// opaque to the domain (a passphrase-sealed copy, a keyring reference…).
type WrappedMasterKey struct {
	kind UnlockerKind
	data []byte
}

// NewWrappedMasterKey records an unlocker's hold on the master key.
func NewWrappedMasterKey(kind UnlockerKind, data []byte) WrappedMasterKey {
	return WrappedMasterKey{kind: kind, data: append([]byte(nil), data...)}
}

// Kind is the unlocker.
func (w WrappedMasterKey) Kind() UnlockerKind { return w.kind }

// Data is the unlocker's own record.
func (w WrappedMasterKey) Data() []byte { return w.data }

// SealedKey is a provider key encrypted under the master key.
type SealedKey struct {
	name   KeyName
	nonce  []byte
	sealed []byte
	setAt  time.Time
}

// NewSealedKey records a sealed key.
func NewSealedKey(name KeyName, nonce, sealed []byte, setAt time.Time) SealedKey {
	return SealedKey{name: name, nonce: append([]byte(nil), nonce...), sealed: append([]byte(nil), sealed...), setAt: setAt.UTC()}
}

// Name is the key's name.
func (k SealedKey) Name() KeyName { return k.name }

// Nonce is the AES-GCM nonce.
func (k SealedKey) Nonce() []byte { return k.nonce }

// Sealed is the ciphertext.
func (k SealedKey) Sealed() []byte { return k.sealed }

// SetAt is when the owner set the key.
func (k SealedKey) SetAt() time.Time { return k.setAt }

// KeyFile is every sealed key and every unlocker's hold on the master key.
// At least one unlocker always holds the master key.
type KeyFile struct {
	unlockers map[UnlockerKind]WrappedMasterKey
	keys      map[KeyName]SealedKey
}

// NewKeyFile starts a key file held by its first unlocker.
func NewKeyFile(first WrappedMasterKey) KeyFile {
	return KeyFile{unlockers: map[UnlockerKind]WrappedMasterKey{first.kind: first}, keys: map[KeyName]SealedKey{}}
}

// RestoreKeyFile rebuilds a stored key file.
func RestoreKeyFile(unlockers []WrappedMasterKey, keys []SealedKey) (KeyFile, error) {
	if len(unlockers) == 0 {
		return KeyFile{}, fmt.Errorf("%w: a key file without unlockers", ErrLocked)
	}
	file := KeyFile{unlockers: map[UnlockerKind]WrappedMasterKey{}, keys: map[KeyName]SealedKey{}}
	for _, wrapped := range unlockers {
		file.unlockers[wrapped.kind] = wrapped
	}
	for _, key := range keys {
		file.keys[key.name] = key
	}
	return file, nil
}

// PutKey adds or replaces a sealed key.
func (f KeyFile) PutKey(key SealedKey) { f.keys[key.name] = key }

// ForgetKey removes a sealed key.
func (f KeyFile) ForgetKey(name KeyName) error {
	if _, ok := f.keys[name]; !ok {
		return fmt.Errorf("%w: %s", ErrKeyNotFound, name)
	}
	delete(f.keys, name)
	return nil
}

// Key finds a sealed key.
func (f KeyFile) Key(name KeyName) (SealedKey, bool) {
	key, ok := f.keys[name]
	return key, ok
}

// Keys lists the sealed keys by name.
func (f KeyFile) Keys() []SealedKey {
	keys := make([]SealedKey, 0, len(f.keys))
	for _, key := range f.keys {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].name < keys[j].name })
	return keys
}

// AddUnlocker records a new hold on the master key.
func (f KeyFile) AddUnlocker(wrapped WrappedMasterKey) error {
	if _, ok := f.unlockers[wrapped.kind]; ok {
		return fmt.Errorf("%w: %s", ErrUnlockerExists, wrapped.kind)
	}
	f.unlockers[wrapped.kind] = wrapped
	return nil
}

// RemoveUnlocker drops an unlocker; the last one cannot go.
func (f KeyFile) RemoveUnlocker(kind UnlockerKind) (WrappedMasterKey, error) {
	wrapped, ok := f.unlockers[kind]
	if !ok {
		return WrappedMasterKey{}, fmt.Errorf("%w: %s", ErrUnlockerMissing, kind)
	}
	if len(f.unlockers) == 1 {
		return WrappedMasterKey{}, ErrLastUnlocker
	}
	delete(f.unlockers, kind)
	return wrapped, nil
}

// Unlockers lists the holds on the master key in UnlockOrder.
func (f KeyFile) Unlockers() []WrappedMasterKey {
	unlockers := make([]WrappedMasterKey, 0, len(f.unlockers))
	for _, kind := range UnlockOrder {
		if wrapped, ok := f.unlockers[kind]; ok {
			unlockers = append(unlockers, wrapped)
		}
	}
	return unlockers
}
