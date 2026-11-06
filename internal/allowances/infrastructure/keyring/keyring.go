// Package keyring implements the allowances KeyRing port with the keys
// context's unseal_key use case: the collector asks for a key by name and
// never sees keys.json, the master key or the unlockers.
package keyring

import (
	"context"
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	keysapp "codeberg.org/simplicer/ataecina/internal/keys/application"
	keysdomain "codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Ring unseals keys for one collection.
type Ring struct {
	unseal *keysapp.UnsealKey
}

// New wraps one process's UnsealKey (the master key opens once).
func New(unseal *keysapp.UnsealKey) Ring { return Ring{unseal: unseal} }

// KeyFor opens the named key, or explains what the owner must do.
func (r Ring) KeyFor(ctx context.Context, name domain.KeyName) (secret.Key, error) {
	keyName, err := keysdomain.NewKeyName(name.String())
	if err != nil {
		return secret.Key{}, domain.KeyMissing(err.Error())
	}
	key, err := r.unseal.Unseal(ctx, keyName)
	switch {
	case err == nil:
		return key, nil
	case errors.Is(err, keysdomain.ErrNotInitialized):
		return secret.Key{}, domain.KeyMissing(fmt.Sprintf("no keys yet: run `ataecina init` and `ataecina keys set %s`", name))
	case errors.Is(err, keysdomain.ErrKeyNotFound):
		return secret.Key{}, domain.KeyMissing(fmt.Sprintf("set the key: ataecina keys set %s", name))
	case errors.Is(err, keysdomain.ErrLocked):
		return secret.Key{}, domain.KeysLocked("no unlocker could open the master key here; see `ataecina doctor`")
	}
	return secret.Key{}, domain.KeysLocked("the keys cannot be read; see `ataecina doctor`")
}
