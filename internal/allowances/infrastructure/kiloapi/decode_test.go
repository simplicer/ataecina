package kiloapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/kiloapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheBalanceIsRead(t *testing.T) {
	balance, err := kiloapi.DecodeBalance(read(t, "balance.json"))
	if err != nil || balance.Left != 4.25 || balance.Depleted {
		t.Fatalf("DecodeBalance = %+v, %v", balance, err)
	}
	empty, err := kiloapi.DecodeBalance([]byte(`{"balance": 0, "isDepleted": true}`))
	if err != nil || empty.Left != 0 || !empty.Depleted {
		t.Fatalf("used up = %+v, %v", empty, err)
	}
	for _, body := range []string{`{}`, `[]`, `{"balance": 1}`, `{"isDepleted": false}`, `{"balance": "1", "isDepleted": false}`, `{"balance": 1e12, "isDepleted": false}`} {
		if _, err := kiloapi.DecodeBalance([]byte(body)); err == nil {
			t.Fatalf("DecodeBalance(%s) accepted", body)
		}
	}
}

// FuzzDecodeBalance checks that no answer panics the decoder.
func FuzzDecodeBalance(f *testing.F) {
	f.Add(read(f, "balance.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = kiloapi.DecodeBalance(body)
	})
}
