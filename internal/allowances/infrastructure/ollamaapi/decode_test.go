package ollamaapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/ollamaapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheSessionAndWeeklyFractions(t *testing.T) {
	limits, err := ollamaapi.DecodeUsage(read(t, "usage.json"))
	if err != nil || limits.Session == nil || *limits.Session != 0.235 || limits.Weekly == nil || *limits.Weekly != 0.41 {
		t.Fatalf("DecodeUsage = %+v, %v", limits, err)
	}
	weeklyOnly, err := ollamaapi.DecodeUsage([]byte(`{"limits":{"weekly":{"usage":0.5}}}`))
	if err != nil || weeklyOnly.Session != nil || *weeklyOnly.Weekly != 0.5 {
		t.Fatalf("weekly only = %+v, %v", weeklyOnly, err)
	}
	for _, body := range []string{`{}`, `{"limits":{}}`, `{"limits":{"session":{}}}`, `{"limits":{"session":{"usage":-0.1}}}`, `[]`} {
		if _, err := ollamaapi.DecodeUsage([]byte(body)); err == nil {
			t.Fatalf("DecodeUsage(%s) accepted", body)
		}
	}
}

// FuzzDecodeUsage checks that no answer panics the decoder.
func FuzzDecodeUsage(f *testing.F) {
	f.Add(read(f, "usage.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		limits, err := ollamaapi.DecodeUsage(body)
		if err == nil && limits.Session == nil && limits.Weekly == nil {
			t.Fatalf("no limit from %q", body)
		}
	})
}
