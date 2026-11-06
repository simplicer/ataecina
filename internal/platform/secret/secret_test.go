package secret_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

const value = "sk-test-0123456789"

func TestAKeyNeverPrintsItself(t *testing.T) {
	key, err := secret.New([]byte(value))
	if err != nil {
		t.Fatal(err)
	}
	wrapped := struct{ Key secret.Key }{key}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("x", "key", key)
	jsonText, _ := json.Marshal(wrapped)
	for _, text := range []string{
		key.String(), key.GoString(), fmt.Sprint(key), fmt.Sprintf("%s %v %+v %#v %x %q", key, key, key, key, key, key),
		fmt.Sprintf("%v %+v %#v", wrapped, wrapped, wrapped), string(jsonText), logged.String(),
		fmt.Errorf("failed with %v", key).Error(),
	} {
		if strings.Contains(text, value) || strings.Contains(text, "736b2d74") {
			t.Fatalf("the key leaked: %s", text)
		}
	}
	if string(key.Reveal()) != value || key.Len() != len(value) {
		t.Fatal("Reveal must give the value back")
	}
}

func TestKeysAreCopiedAndWiped(t *testing.T) {
	raw := []byte(value)
	key, _ := secret.New(raw)
	raw[0] = 'X'
	if key.Reveal()[0] != 's' {
		t.Fatal("New must copy the bytes")
	}
	key.Wipe()
	if !bytes.Equal(key.Reveal(), make([]byte, len(value))) {
		t.Fatal("Wipe must zero the bytes")
	}
	if _, err := secret.New(nil); !errors.Is(err, secret.ErrEmpty) {
		t.Fatalf("New(nil) = %v", err)
	}
	if text, _ := key.MarshalText(); string(text) != "[REDACTED]" {
		t.Fatal("MarshalText must redact")
	}
}
