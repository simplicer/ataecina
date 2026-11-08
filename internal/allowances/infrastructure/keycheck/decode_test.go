package keycheck_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/keycheck"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAGoodAnswerHoldsItsList(t *testing.T) {
	for _, c := range []struct {
		fixture string
		list    string
	}{
		{"groq-models.json", "data"},
		{"nvidia-functions.json", "functions"},
		{"gemini-models.json", "models"},
	} {
		if err := keycheck.DecodeList(read(t, c.fixture), c.list); err != nil {
			t.Fatalf("%s: %v", c.fixture, err)
		}
	}
	for _, body := range []string{`{}`, `[]`, `{"data": null}`, `{"data": {}}`, `{"data": "list"}`, `<html>`, ``} {
		if err := keycheck.DecodeList([]byte(body), "data"); err == nil {
			t.Fatalf("DecodeList(%s) accepted", body)
		}
	}
}

// FuzzDecodeList checks that no answer panics the decoder.
func FuzzDecodeList(f *testing.F) {
	f.Add(read(f, "groq-models.json"))
	f.Add(read(f, "nvidia-functions.json"))
	f.Add(read(f, "gemini-models.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_ = keycheck.DecodeList(body, "data")
	})
}
