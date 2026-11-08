package widgetcatalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/widgetcatalog"
)

const po = `# Spanish.
msgid ""
msgstr ""
"Language: es\n"

#: a.qml:1
msgid "Refresh now"
msgstr "Actualizar ahora"

msgid "A long "
"sentence"
msgstr "Una frase "
"larga"

#, fuzzy
msgid "Guessed"
msgstr "Adivinada"

msgid "Untranslated"
msgstr ""

msgid "%1 model"
msgid_plural "%1 models"
msgstr[0] "%1 modelo"
msgstr[1] "%1 modelos"
`

func TestTheTableHoldsEveryTranslatedMessage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "es.po"), []byte(po), 0o600); err != nil {
		t.Fatal(err)
	}
	table, err := widgetcatalog.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	text := string(table)
	for _, want := range []string{widgetcatalog.Header, `"es": {`, `"Refresh now": "Actualizar ahora"`, `"A long sentence": "Una frase larga"`, `"%1 modelo"`, `"%1 modelos"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Guessed", "Untranslated", `"": `} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("%q should be left out:\n%s", unwanted, text)
		}
	}
}

func TestABrokenFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "es.po"), []byte("msgid unquoted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetcatalog.Build(dir); err == nil {
		t.Fatal("a broken PO file was read")
	}
	if err := os.WriteFile(filepath.Join(dir, "es.po"), []byte("\"stray\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetcatalog.Build(dir); err == nil {
		t.Fatal("a stray string was read")
	}
}
