package xaiapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/xaiapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheTotalIsReadInCents(t *testing.T) {
	if cents, err := xaiapi.DecodeBalance(read(t, "balance.json")); err != nil || cents != -1750 {
		t.Fatalf("DecodeBalance = %v, %v", cents, err)
	}
	for _, body := range []string{`{}`, `[]`, `{"total":{}}`, `{"total":{"val":"ten"}}`, `{"total":{"val":-5}}`, `{"total":{"val":"1e20"}}`, `{"total":{"val":"NaN"}}`} {
		if _, err := xaiapi.DecodeBalance([]byte(body)); err == nil {
			t.Fatalf("DecodeBalance(%s) accepted", body)
		}
	}
}

// FuzzDecodeBalance checks that no answer panics the decoder.
func FuzzDecodeBalance(f *testing.F) {
	f.Add(read(f, "balance.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = xaiapi.DecodeBalance(body)
	})
}
