package application

import "codeberg.org/simplicer/ataecina/internal/keys/domain"

// ForgetKey removes a sealed key (`ataecina keys forget`); no unlocking is
// needed to forget.
type ForgetKey struct {
	files domain.KeyFiles
}

// NewForgetKey prepares the removal.
func NewForgetKey(files domain.KeyFiles) ForgetKey { return ForgetKey{files: files} }

// Forget removes the key with that name.
func (f ForgetKey) Forget(name domain.KeyName) error {
	file, err := f.files.Load()
	if err != nil {
		return err
	}
	if err := file.ForgetKey(name); err != nil {
		return err
	}
	return f.files.Save(file)
}
