package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
)

// AddUnlocker gives the master key to one more unlocker
// (`ataecina unlockers add`), after opening it with an existing one.
type AddUnlocker struct {
	files     domain.KeyFiles
	unlockers domain.Unlockers
}

// NewAddUnlocker prepares the addition.
func NewAddUnlocker(files domain.KeyFiles, unlockers domain.Unlockers) AddUnlocker {
	return AddUnlocker{files: files, unlockers: unlockers}
}

// Add wraps the master key for the given unlocker.
func (a AddUnlocker) Add(ctx context.Context, kind domain.UnlockerKind) error {
	file, err := a.files.Load()
	if err != nil {
		return err
	}
	for _, existing := range file.Unlockers() {
		if existing.Kind() == kind {
			return fmt.Errorf("%w: %s", domain.ErrUnlockerExists, kind)
		}
	}
	unlocker, available := a.unlockers[kind]
	if !available {
		return fmt.Errorf("%w: %s", domain.ErrUnlockerUnavailable, kind)
	}
	master, err := openMasterKey(ctx, file, a.unlockers)
	if err != nil {
		return err
	}
	defer master.Wipe()
	wrapped, err := unlocker.Wrap(ctx, master)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	if err := file.AddUnlocker(wrapped); err != nil {
		return err
	}
	return a.files.Save(file)
}
