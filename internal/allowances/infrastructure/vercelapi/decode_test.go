package vercelapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/vercelapi"
)

func credits(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/credits.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheBalanceAndTheSpend(t *testing.T) {
	balance, err := vercelapi.DecodeCredits(credits(t))
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := balance.Remaining()
	spent, _ := balance.Spent()
	if remaining.Amount().String() != "95.5" || spent.Amount().String() != "4.5" || remaining.Currency() != "USD" {
		t.Fatalf("balance = %v %v", remaining, spent)
	}
	if _, err := vercelapi.DecodeCredits([]byte(`{"balance": 3.25, "total_used": 1}`)); err != nil {
		t.Fatalf("numbers are accepted too: %v", err)
	}
	for _, body := range []string{`{"balance":"x","total_used":"1"}`, `{"balance":"1","total_used":"y"}`, `{"balance":"1"}`, `[]`} {
		if _, err := vercelapi.DecodeCredits([]byte(body)); err == nil {
			t.Fatalf("DecodeCredits(%s) accepted", body)
		}
	}
}

// FuzzDecodeCredits checks that no answer panics the decoder.
func FuzzDecodeCredits(f *testing.F) {
	f.Add(credits(f))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = vercelapi.DecodeCredits(body)
	})
}
