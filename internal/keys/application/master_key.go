// Package application holds the keys use cases, one per file.
package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// openMasterKey tries the file's unlockers in domain.UnlockOrder, among
// those this process can use, and returns the first master key one gives.
func openMasterKey(ctx context.Context, file domain.KeyFile, unlockers domain.Unlockers) (secret.Key, error) {
	var last error
	for _, wrapped := range file.Unlockers() {
		unlocker, available := unlockers[wrapped.Kind()]
		if !available {
			continue
		}
		master, err := unlocker.Unwrap(ctx, wrapped)
		if err == nil {
			return master, nil
		}
		last = fmt.Errorf("%s: %w", wrapped.Kind(), err)
	}
	if last != nil {
		return secret.Key{}, fmt.Errorf("%w (%w)", domain.ErrLocked, last)
	}
	return secret.Key{}, domain.ErrLocked
}
