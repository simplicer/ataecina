// Package keyfile keeps $XDG_CONFIG_HOME/ataecina/keys.json (0600, in a
// 0700 directory): every sealed key and every unlocker's hold on the
// master key, never a key or the master key in clear (SPECS §6).
package keyfile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	formatVersion = 1
	maxBytes      = 1 << 20
)

type document struct {
	Version   int                        `json:"version"`
	MasterKey map[string]json.RawMessage `json:"master_key"`
	Keys      map[string]sealedDocument  `json:"keys"`
}

type sealedDocument struct {
	Nonce  string    `json:"nonce"`
	Sealed string    `json:"sealed"`
	SetAt  time.Time `json:"set_at"`
}

// Store reads and writes keys.json.
type Store struct {
	path string
}

// NewStore keeps the key file at path.
func NewStore(path string) Store { return Store{path: path} }

// Load reads the key file, or domain.ErrNotInitialized.
func (s Store) Load() (domain.KeyFile, error) {
	if err := privatefs.RequireDir(filepath.Dir(s.path)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.KeyFile{}, domain.ErrNotInitialized
		}
		return domain.KeyFile{}, err
	}
	data, err := privatefs.ReadFile(s.path, maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.KeyFile{}, domain.ErrNotInitialized
	}
	if err != nil {
		return domain.KeyFile{}, err
	}
	var stored document
	if err := json.Unmarshal(data, &stored); err != nil || stored.Version != formatVersion {
		return domain.KeyFile{}, fmt.Errorf("%s is not a key file Ataecina can read", s.path)
	}
	unlockers := make([]domain.WrappedMasterKey, 0, len(stored.MasterKey))
	for kind, data := range stored.MasterKey {
		parsed, err := domain.ParseUnlockerKind(kind)
		if err != nil {
			return domain.KeyFile{}, fmt.Errorf("%s: %w", s.path, err)
		}
		unlockers = append(unlockers, domain.NewWrappedMasterKey(parsed, data))
	}
	keys := make([]domain.SealedKey, 0, len(stored.Keys))
	for name, item := range stored.Keys {
		key, err := decodeSealed(name, item)
		if err != nil {
			return domain.KeyFile{}, fmt.Errorf("%s: key %q: %w", s.path, name, err)
		}
		keys = append(keys, key)
	}
	return domain.RestoreKeyFile(unlockers, keys)
}

// Save writes the key file atomically, creating its private directory.
func (s Store) Save(file domain.KeyFile) error {
	stored := document{Version: formatVersion, MasterKey: map[string]json.RawMessage{}, Keys: map[string]sealedDocument{}}
	for _, wrapped := range file.Unlockers() {
		data := wrapped.Data()
		if !json.Valid(data) {
			return fmt.Errorf("the %s record is not JSON", wrapped.Kind())
		}
		stored.MasterKey[string(wrapped.Kind())] = data
	}
	for _, key := range file.Keys() {
		stored.Keys[key.Name().String()] = sealedDocument{
			Nonce:  base64.StdEncoding.EncodeToString(key.Nonce()),
			Sealed: base64.StdEncoding.EncodeToString(key.Sealed()),
			SetAt:  key.SetAt().UTC().Truncate(time.Second),
		}
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := privatefs.EnsureDir(filepath.Dir(s.path)); err != nil {
		return err
	}
	return privatefs.WriteFile(s.path, append(data, '\n'))
}

func decodeSealed(name string, item sealedDocument) (domain.SealedKey, error) {
	keyName, err := domain.NewKeyName(name)
	if err != nil {
		return domain.SealedKey{}, err
	}
	nonce, err := base64.StdEncoding.DecodeString(item.Nonce)
	if err != nil {
		return domain.SealedKey{}, err
	}
	sealed, err := base64.StdEncoding.DecodeString(item.Sealed)
	if err != nil {
		return domain.SealedKey{}, err
	}
	return domain.NewSealedKey(keyName, nonce, sealed, item.SetAt), nil
}
