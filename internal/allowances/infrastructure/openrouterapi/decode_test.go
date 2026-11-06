package openrouterapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/openrouterapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheKeyAndTheCredits(t *testing.T) {
	facts, err := openrouterapi.DecodeKey(read(t, "key.json"))
	if err != nil || facts.Usage != 11.58 || facts.Limit != nil || facts.FreeUsed != 12 || facts.FreeLimit != 50 {
		t.Fatalf("DecodeKey = %+v, %v", facts, err)
	}
	credits, err := openrouterapi.DecodeCredits(read(t, "credits.json"))
	if err != nil || credits.Total != 20 || credits.Used != 11.58 {
		t.Fatalf("DecodeCredits = %+v, %v", credits, err)
	}
	for _, body := range []string{`{"data":{}}`, `{"data":{"usage":1,"free_model_daily_requests":{"used":-1,"limit":5}}}`, `[]`} {
		if _, err := openrouterapi.DecodeKey([]byte(body)); err == nil {
			t.Fatalf("DecodeKey(%s) accepted", body)
		}
	}
	for _, body := range []string{`{"data":{"total_credits":1}}`, `[]`} {
		if _, err := openrouterapi.DecodeCredits([]byte(body)); err == nil {
			t.Fatalf("DecodeCredits(%s) accepted", body)
		}
	}
}

// FuzzDecodeKey checks that no answer panics the decoder.
func FuzzDecodeKey(f *testing.F) {
	f.Add(read(f, "key.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		facts, err := openrouterapi.DecodeKey(body)
		if err == nil && (facts.FreeUsed < 0 || facts.FreeLimit < 0) {
			t.Fatalf("negative free requests from %q", body)
		}
	})
}
