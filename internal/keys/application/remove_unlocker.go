package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
)

// RemoveUnlocker drops one unlocker (`ataecina unlockers remove`); at
// least one must remain. The unlocker's own copy (a keyring item, a
// credential file) is forgotten when this process can reach it.
type RemoveUnlocker struct {
	files     domain.KeyFiles
	unlockers domain.Unlockers
}

// NewRemoveUnlocker prepares the removal.
func NewRemoveUnlocker(files domain.KeyFiles, unlockers domain.Unlockers) RemoveUnlocker {
	return RemoveUnlocker{files: files, unlockers: unlockers}
}

// Remove drops the given unlocker.
func (r RemoveUnlocker) Remove(ctx context.Context, kind domain.UnlockerKind) error {
	file, err := r.files.Load()
	if err != nil {
		return err
	}
	wrapped, err := file.RemoveUnlocker(kind)
	if err != nil {
		return err
	}
	if err := r.files.Save(file); err != nil {
		return err
	}
	unlocker, available := r.unlockers[kind]
	if !available {
		return nil
	}
	if err := unlocker.Forget(ctx, wrapped); err != nil {
		return fmt.Errorf("removed from keys.json, but %s still holds a copy: %w", kind, err)
	}
	return nil
}
