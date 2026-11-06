package application

import "codeberg.org/simplicer/ataecina/internal/keys/domain"

// ListUnlockers tells which unlockers hold the master key, in the order
// they are tried.
type ListUnlockers struct {
	files domain.KeyFiles
}

// NewListUnlockers prepares the listing.
func NewListUnlockers(files domain.KeyFiles) ListUnlockers { return ListUnlockers{files: files} }

// List returns the unlocker kinds.
func (l ListUnlockers) List() ([]domain.UnlockerKind, error) {
	file, err := l.files.Load()
	if err != nil {
		return nil, err
	}
	kinds := make([]domain.UnlockerKind, 0, 3)
	for _, wrapped := range file.Unlockers() {
		kinds = append(kinds, wrapped.Kind())
	}
	return kinds, nil
}
