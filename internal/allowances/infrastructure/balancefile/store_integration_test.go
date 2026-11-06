//go:build integration

package balancefile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/balancefile"
)

var at = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func dollars(t *testing.T, raw string) domain.Money {
	t.Helper()
	amount, _ := domain.ParseAmount(raw)
	usd, _ := domain.NewCurrency("USD")
	return domain.NewMoney(amount, usd)
}

func declaration(t *testing.T, kind domain.DeclarationKind, amount string) domain.Declaration {
	t.Helper()
	d, err := domain.NewDeclaration(kind, dollars(t, amount), at, dollars(t, "1.5"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDeclarationsAreKeptPrivatelyPerSubscription(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "ataecina")
	store := balancefile.NewStore(dir)
	if got, err := store.DeclarationsOf("zen"); err != nil || len(got) != 0 {
		t.Fatalf("before any declaration = %v, %v", got, err)
	}
	for _, d := range []domain.Declaration{
		declaration(t, domain.DeclaredTopUp, "20"),
		declaration(t, domain.DeclaredTopUp, "10"),
	} {
		if err := store.KeepDeclaration("zen", d); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.KeepDeclaration("other", declaration(t, domain.DeclaredTopUp, "1")); err != nil {
		t.Fatal(err)
	}
	got, err := store.DeclarationsOf("zen")
	if err != nil || len(got) != 2 || got[1].Amount().Amount().String() != "10" || got[0].SpentBaseline().Amount().String() != "1.5" || !got[0].DeclaredAt().Equal(at) {
		t.Fatalf("declarations = %+v, %v", got, err)
	}
	if err := store.KeepDeclaration("zen", declaration(t, domain.DeclaredBalance, "7")); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.DeclarationsOf("zen"); len(got) != 1 || got[0].Kind() != domain.DeclaredBalance {
		t.Fatalf("a declared balance replaces what came before: %+v", got)
	}
	info, err := os.Stat(filepath.Join(dir, "balances.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("balances.json = %v", err)
	}
}

func TestACorruptOrOverfullFileIsRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store := balancefile.NewStore(dir)
	for range 100 {
		if err := store.KeepDeclaration("zen", declaration(t, domain.DeclaredTopUp, "1")); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.KeepDeclaration("zen", declaration(t, domain.DeclaredTopUp, "1")); err == nil {
		t.Fatal("the 101st top-up must be refused")
	}
	for name, content := range map[string]string{
		"not json":     `{`,
		"old version":  `{"version": 7, "subscriptions": {}}`,
		"bad kind":     `{"version": 1, "subscriptions": {"zen": [{"kind": "GIFT", "amount": {"amount": 1, "currency": "USD"}, "declared_at": "2026-11-06T19:40:00Z", "spent_baseline": {"amount": 0, "currency": "USD"}}]}}`,
		"bad amount":   `{"version": 1, "subscriptions": {"zen": [{"kind": "TOP_UP", "amount": {"amount": 1, "currency": "usd"}, "declared_at": "2026-11-06T19:40:00Z", "spent_baseline": {"amount": 0, "currency": "USD"}}]}}`,
		"bad baseline": `{"version": 1, "subscriptions": {"zen": [{"kind": "TOP_UP", "amount": {"amount": 1, "currency": "USD"}, "declared_at": "2026-11-06T19:40:00Z", "spent_baseline": {"amount": "x", "currency": "USD"}}]}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "balances.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DeclarationsOf("zen"); err == nil {
			t.Fatalf("%s: accepted", name)
		}
		if name == "not json" {
			if err := store.KeepDeclaration("zen", declaration(t, domain.DeclaredTopUp, "1")); err == nil {
				t.Fatal("a corrupt file must not be overwritten")
			}
		}
	}
	if err := os.Chmod(filepath.Join(dir, "balances.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeclarationsOf("zen"); err == nil {
		t.Fatal("a loose balances file must be refused")
	}
}
