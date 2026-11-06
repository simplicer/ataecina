package application

import (
	"context"
	"errors"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
)

// InitializeKeys creates the master key and hands it to its first unlocker
// (`ataecina init`). The master key is never written in clear.
type InitializeKeys struct {
	files     domain.KeyFiles
	cipher    domain.Cipher
	unlockers domain.Unlockers
}

// NewInitializeKeys prepares the initialisation.
func NewInitializeKeys(files domain.KeyFiles, cipher domain.Cipher, unlockers domain.Unlockers) InitializeKeys {
	return InitializeKeys{files: files, cipher: cipher, unlockers: unlockers}
}

// Initialize creates keys.json held by the given unlocker.
func (i InitializeKeys) Initialize(ctx context.Context, kind domain.UnlockerKind) error {
	_, err := i.files.Load()
	if err == nil {
		return domain.ErrAlreadyInitialized
	}
	if !errors.Is(err, domain.ErrNotInitialized) {
		return err
	}
	unlocker, available := i.unlockers[kind]
	if !available {
		return fmt.Errorf("%w: %s", domain.ErrUnlockerUnavailable, kind)
	}
	master, err := i.cipher.NewMasterKey()
	if err != nil {
		return fmt.Errorf("create the master key: %w", err)
	}
	defer master.Wipe()
	wrapped, err := unlocker.Wrap(ctx, master)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	return i.files.Save(domain.NewKeyFile(wrapped))
}
