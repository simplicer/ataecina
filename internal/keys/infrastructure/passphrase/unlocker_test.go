package passphrase_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/aesgcm"
	"codeberg.org/simplicer/ataecina/internal/keys/infrastructure/passphrase"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Tests use few iterations; production uses passphrase.Iterations.
const fast = 1000

func lines(text string) passphrase.Ask { return passphrase.FromReader(strings.NewReader(text)) }

func TestTheMasterKeyIsWrappedUnderThePassphrase(t *testing.T) {
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	wrapped, err := passphrase.New(lines("correct horse battery\n"), fast).Wrap(context.Background(), master)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped.Kind() != domain.Passphrase || bytes.Contains(wrapped.Data(), master.Reveal()) || !bytes.Contains(wrapped.Data(), []byte(`"kdf":"pbkdf2-sha256"`)) {
		t.Fatalf("record = %s", wrapped.Data())
	}
	opened, err := passphrase.New(lines("correct horse battery\n"), fast).Unwrap(context.Background(), wrapped)
	if err != nil || !bytes.Equal(opened.Reveal(), master.Reveal()) {
		t.Fatalf("Unwrap = %v", err)
	}
	if _, err := passphrase.New(lines("wrong horse battery\n"), fast).Unwrap(context.Background(), wrapped); !errors.Is(err, domain.ErrBrokenSeal) {
		t.Fatalf("wrong passphrase = %v", err)
	}
	if err := passphrase.New(lines(""), fast).Forget(context.Background(), wrapped); err != nil {
		t.Fatal(err)
	}
}

func TestWeakOrMissingPassphrasesAreRefused(t *testing.T) {
	master, _ := aesgcm.Cipher{}.NewMasterKey()
	if _, err := passphrase.New(lines("short\n"), fast).Wrap(context.Background(), master); err == nil {
		t.Fatal("a short passphrase must be refused")
	}
	for _, input := range []string{"", "   \n"} {
		if _, err := passphrase.New(lines(input), fast).Wrap(context.Background(), master); err == nil {
			t.Fatalf("passphrase %q accepted", input)
		}
	}
	for _, record := range []string{`not json`, `{"kdf":"scrypt","iterations":1}`, `{"kdf":"pbkdf2-sha256","iterations":0}`} {
		wrapped := domain.NewWrappedMasterKey(domain.Passphrase, []byte(record))
		if _, err := passphrase.New(lines("whatever-long\n"), fast).Unwrap(context.Background(), wrapped); err == nil {
			t.Fatalf("record %s accepted", record)
		}
	}
	wrapped, _ := passphrase.New(lines("correct horse battery\n"), fast).Wrap(context.Background(), master)
	if _, err := passphrase.New(lines(""), fast).Unwrap(context.Background(), wrapped); err == nil {
		t.Fatal("no passphrase must fail")
	}
	failing := func(string, bool) (secret.Key, error) { return secret.Key{}, domain.ErrUnlockerUnavailable }
	if _, err := passphrase.New(failing, fast).Wrap(context.Background(), master); !errors.Is(err, domain.ErrUnlockerUnavailable) {
		t.Fatalf("no terminal = %v", err)
	}
}

func TestOneLinePerQuestionFromADescriptor(t *testing.T) {
	ask := lines("first-passphrase\nsecond-passphrase")
	for _, want := range []string{"first-passphrase", "second-passphrase"} {
		got, err := ask("", false)
		if err != nil || string(got.Reveal()) != want {
			t.Fatalf("ask = %v", err)
		}
	}
}
