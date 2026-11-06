package application

import (
	"context"
	"fmt"
	"sync"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// UnsealKey opens one provider key for the collector, through the
// KeyRing port of allowances. The master key is unlocked at most once per
// process (one keyring prompt, one passphrase), even when sources ask
// concurrently, and the outcome is remembered.
type UnsealKey struct {
	files     domain.KeyFiles
	cipher    domain.Cipher
	unlockers domain.Unlockers

	once   sync.Once
	file   domain.KeyFile
	master secret.Key
	failed error
}

// NewUnsealKey prepares the unsealing for one process.
func NewUnsealKey(files domain.KeyFiles, cipher domain.Cipher, unlockers domain.Unlockers) *UnsealKey {
	return &UnsealKey{files: files, cipher: cipher, unlockers: unlockers}
}

// Unseal opens the key with that name.
func (u *UnsealKey) Unseal(ctx context.Context, name domain.KeyName) (secret.Key, error) {
	u.once.Do(func() {
		u.file, u.failed = u.files.Load()
		if u.failed == nil {
			u.master, u.failed = openMasterKey(ctx, u.file, u.unlockers)
		}
	})
	if u.failed != nil {
		return secret.Key{}, u.failed
	}
	sealed, found := u.file.Key(name)
	if !found {
		return secret.Key{}, fmt.Errorf("%w: %s", domain.ErrKeyNotFound, name)
	}
	return u.cipher.Open(u.master, sealed)
}
