package application_test

import (
	"bytes"
	"context"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

var now = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// shelf is an in-memory keys.json.
type shelf struct {
	file    *domain.KeyFile
	loadErr error
	saveErr error
	saves   int
}

func (s *shelf) Load() (domain.KeyFile, error) {
	if s.loadErr != nil {
		return domain.KeyFile{}, s.loadErr
	}
	if s.file == nil {
		return domain.KeyFile{}, domain.ErrNotInitialized
	}
	return *s.file, nil
}

func (s *shelf) Save(file domain.KeyFile) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.file = &file
	return nil
}

var masterBytes = []byte("0123456789abcdef0123456789abcdef")

// cipher "seals" by storing the key next to the master key it used.
type cipher struct {
	newErr  error
	sealErr error
}

func (c cipher) NewMasterKey() (secret.Key, error) {
	if c.newErr != nil {
		return secret.Key{}, c.newErr
	}
	return secret.New(masterBytes)
}

func (c cipher) Seal(master secret.Key, name domain.KeyName, key secret.Key, setAt time.Time) (domain.SealedKey, error) {
	if c.sealErr != nil {
		return domain.SealedKey{}, c.sealErr
	}
	return domain.NewSealedKey(name, master.Reveal(), key.Reveal(), setAt), nil
}

func (cipher) Open(master secret.Key, sealed domain.SealedKey) (secret.Key, error) {
	if !bytes.Equal(master.Reveal(), sealed.Nonce()) {
		return secret.Key{}, domain.ErrBrokenSeal
	}
	return secret.New(sealed.Sealed())
}

// unlocker keeps the master key in its wrapped data.
type unlocker struct {
	kind      domain.UnlockerKind
	wrapErr   error
	unwrapErr error
	forgetErr error
	unwraps   *int
	forgotten *bool
}

func (u unlocker) Wrap(_ context.Context, master secret.Key) (domain.WrappedMasterKey, error) {
	if u.wrapErr != nil {
		return domain.WrappedMasterKey{}, u.wrapErr
	}
	return domain.NewWrappedMasterKey(u.kind, master.Reveal()), nil
}

func (u unlocker) Unwrap(_ context.Context, wrapped domain.WrappedMasterKey) (secret.Key, error) {
	if u.unwraps != nil {
		*u.unwraps++
	}
	if u.unwrapErr != nil {
		return secret.Key{}, u.unwrapErr
	}
	return secret.New(wrapped.Data())
}

func (u unlocker) Forget(context.Context, domain.WrappedMasterKey) error {
	if u.forgotten != nil {
		*u.forgotten = true
	}
	return u.forgetErr
}

func initialised(kinds ...domain.UnlockerKind) *shelf {
	file := domain.NewKeyFile(domain.NewWrappedMasterKey(kinds[0], masterBytes))
	for _, kind := range kinds[1:] {
		_ = file.AddUnlocker(domain.NewWrappedMasterKey(kind, masterBytes))
	}
	return &shelf{file: &file}
}

func key(text string) secret.Key {
	k, _ := secret.New([]byte(text))
	return k
}

var errBoom = errors.New("boom")
