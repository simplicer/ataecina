package ollamaapi_test

import (
	"os"
	"testing"
	"time"

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

func TestACreditPlanGivesItsAllowanceAndRenewal(t *testing.T) {
	balance, err := ollamaapi.DecodeBalance(read(t, "balance.json"))
	if err != nil {
		t.Fatal(err)
	}
	included := balance.Included
	if included == nil || included.Left != 45 || included.Allowance != 60 || !included.Until.Equal(time.Date(2026, 11, 20, 8, 15, 0, 0, time.UTC)) {
		t.Fatalf("included = %+v", included)
	}
	if balance.Purchased == nil || *balance.Purchased != 5.5 || balance.Session != nil {
		t.Fatalf("balance = %+v", balance)
	}
}

func TestALegacyPlanGivesItsWindows(t *testing.T) {
	balance, err := ollamaapi.DecodeBalance(read(t, "legacy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if balance.Included != nil || balance.Session == nil || balance.Session.LeftPercent != 76.5 || balance.Weekly == nil || !balance.Weekly.ResetsAt.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("balance = %+v", balance)
	}
	purchasedOnly, err := ollamaapi.DecodeBalance([]byte(`{"included": null, "purchased": {"balance_usd": 3}}`))
	if err != nil || purchasedOnly.Purchased == nil || *purchasedOnly.Purchased != 3 {
		t.Fatalf("pay as you go = %+v, %v", purchasedOnly, err)
	}
}

func TestAChangedBalanceIsRefused(t *testing.T) {
	for _, body := range []string{
		`{}`, `[]`, `{"included":{},"purchased":{}}`,
		`{"limits":{"session":{"usage":0.2}}}`,
		`{"included":{"balance_usd":10}}`,
		`{"included":{"balance_usd":-1,"allowance_usd":60}}`,
		`{"included":{"balance_usd":10,"allowance_usd":60,"period":{"until":"next month"}}}`,
		`{"included":{"session":{"remaining_percent":120}}}`,
		`{"included":{"weekly":{"resets_at":"2026-11-09T00:00:00Z"}}}`,
		`{"included":{"session":{"remaining_percent":10,"resets_at":"soon"}}}`,
		`{"purchased":{"balance_usd":-2}}`,
	} {
		if _, err := ollamaapi.DecodeBalance([]byte(body)); err == nil {
			t.Fatalf("DecodeBalance(%s) accepted", body)
		}
	}
}

// FuzzDecodeBalance checks that no answer panics the decoder.
func FuzzDecodeBalance(f *testing.F) {
	f.Add(read(f, "balance.json"))
	f.Add(read(f, "legacy.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		balance, err := ollamaapi.DecodeBalance(body)
		if err != nil {
			return
		}
		if included := balance.Included; included != nil && (included.Left < 0 || included.Allowance < 0) {
			t.Fatalf("negative money from %q", body)
		}
	})
}
