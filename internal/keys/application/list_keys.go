package application

import "codeberg.org/simplicer/ataecina/internal/keys/domain"

// ListKeys lists the sealed keys with the date each was set, never a key.
type ListKeys struct {
	files domain.KeyFiles
}

// NewListKeys prepares the listing.
func NewListKeys(files domain.KeyFiles) ListKeys { return ListKeys{files: files} }

// List returns the sealed keys by name.
func (l ListKeys) List() ([]domain.SealedKey, error) {
	file, err := l.files.Load()
	if err != nil {
		return nil, err
	}
	return file.Keys(), nil
}
