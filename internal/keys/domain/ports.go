package domain

import (
	"context"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// KeyFiles keeps the key file; Load returns ErrNotInitialized when there
// is none yet.
type KeyFiles interface {
	Load() (KeyFile, error)
	Save(file KeyFile) error
}

// Cipher makes master keys and seals provider keys under them, binding
// each to its name (AssociatedData).
type Cipher interface {
	NewMasterKey() (secret.Key, error)
	Seal(master secret.Key, name KeyName, key secret.Key, setAt time.Time) (SealedKey, error)
	Open(master secret.Key, sealed SealedKey) (secret.Key, error)
}

// Unlocker holds the master key in one way. Unwrap returns
// ErrUnlockerUnavailable when it cannot work in this process (no desktop
// session, no terminal, not under systemd).
type Unlocker interface {
	Wrap(ctx context.Context, master secret.Key) (WrappedMasterKey, error)
	Unwrap(ctx context.Context, wrapped WrappedMasterKey) (secret.Key, error)
	Forget(ctx context.Context, wrapped WrappedMasterKey) error
}

// Unlockers are the unlockers this process can use, by kind.
type Unlockers map[UnlockerKind]Unlocker

// Clock tells the time; the domain itself never reads it.
type Clock interface {
	Now() time.Time
}
