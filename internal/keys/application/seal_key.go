package application

import (
	"context"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// SealKey seals a provider key under the master key (`ataecina keys set`).
type SealKey struct {
	files     domain.KeyFiles
	cipher    domain.Cipher
	unlockers domain.Unlockers
	clock     domain.Clock
}

// NewSealKey prepares the sealing.
func NewSealKey(files domain.KeyFiles, cipher domain.Cipher, unlockers domain.Unlockers, clock domain.Clock) SealKey {
	return SealKey{files: files, cipher: cipher, unlockers: unlockers, clock: clock}
}

// Seal adds or replaces the key with that name.
func (s SealKey) Seal(ctx context.Context, name domain.KeyName, key secret.Key) error {
	file, err := s.files.Load()
	if err != nil {
		return err
	}
	master, err := openMasterKey(ctx, file, s.unlockers)
	if err != nil {
		return err
	}
	defer master.Wipe()
	sealed, err := s.cipher.Seal(master, name, key, s.clock.Now())
	if err != nil {
		return err
	}
	file.PutKey(sealed)
	return s.files.Save(file)
}
