// Package balancefile keeps the owner's declared balances in
// $XDG_STATE_HOME/ataecina/balances.json (0600).
package balancefile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/readingjson"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const (
	formatVersion = 1
	maxBytes      = 1 << 20
	// maxDeclarations bounds the top-ups kept after the latest balance.
	maxDeclarations = 100
)

type document struct {
	Version       int                              `json:"version"`
	Subscriptions map[string][]declarationDocument `json:"subscriptions"`
}

type declarationDocument struct {
	Kind          string              `json:"kind"`
	Amount        readingjson.Money   `json:"amount"`
	DeclaredAt    readingjson.Instant `json:"declared_at"`
	SpentBaseline readingjson.Money   `json:"spent_baseline"`
}

// Store reads and writes balances.json in the state directory.
type Store struct {
	dir string
}

// NewStore keeps the declarations in dir.
func NewStore(dir string) Store { return Store{dir: dir} }

func (s Store) path() string { return filepath.Join(s.dir, "balances.json") }

// DeclarationsOf returns a subscription's declarations, oldest first.
func (s Store) DeclarationsOf(subscription domain.SubscriptionID) ([]domain.Declaration, error) {
	stored, err := s.load()
	if err != nil {
		return nil, err
	}
	declarations := make([]domain.Declaration, 0, len(stored.Subscriptions[subscription.String()]))
	for _, item := range stored.Subscriptions[subscription.String()] {
		declaration, err := decode(item)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.path(), err)
		}
		declarations = append(declarations, declaration)
	}
	return declarations, nil
}

// KeepDeclaration appends a declaration; a declared balance replaces the
// declarations before it, which no longer count.
func (s Store) KeepDeclaration(subscription domain.SubscriptionID, declaration domain.Declaration) error {
	stored, err := s.load()
	if err != nil {
		return err
	}
	items := stored.Subscriptions[subscription.String()]
	if declaration.Kind() == domain.DeclaredBalance {
		items = nil
	}
	items = append(items, encode(declaration))
	if len(items) > maxDeclarations {
		return fmt.Errorf("%s: more than %d top-ups since the last declared balance; declare the current balance instead", s.path(), maxDeclarations)
	}
	stored.Subscriptions[subscription.String()] = items
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	return privatefs.WriteFile(s.path(), append(data, '\n'))
}

func (s Store) load() (document, error) {
	stored := document{Version: formatVersion, Subscriptions: map[string][]declarationDocument{}}
	data, err := privatefs.ReadFile(s.path(), maxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return stored, nil
	}
	if err != nil {
		return document{}, err
	}
	if err := json.Unmarshal(data, &stored); err != nil || stored.Version != formatVersion || stored.Subscriptions == nil {
		return document{}, fmt.Errorf("%s is not a balances file Ataecina can read", s.path())
	}
	return stored, nil
}

func encode(declaration domain.Declaration) declarationDocument {
	return declarationDocument{
		Kind:          string(declaration.Kind()),
		Amount:        *readingjson.EncodeMoney(declaration.Amount()),
		DeclaredAt:    readingjson.Instant{Time: declaration.DeclaredAt()},
		SpentBaseline: *readingjson.EncodeMoney(declaration.SpentBaseline()),
	}
}

func decode(item declarationDocument) (domain.Declaration, error) {
	kind, err := domain.ParseDeclarationKind(item.Kind)
	if err != nil {
		return domain.Declaration{}, err
	}
	amount, err := readingjson.DecodeMoney(&item.Amount)
	if err != nil {
		return domain.Declaration{}, err
	}
	baseline, err := readingjson.DecodeMoney(&item.SpentBaseline)
	if err != nil {
		return domain.Declaration{}, err
	}
	return domain.NewDeclaration(kind, *amount, item.DeclaredAt.Time, *baseline)
}
