package moonshotapi_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/moonshotapi"
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
	balance, err := moonshotapi.DecodeBalance(read(t, "balance.json"))
	if err != nil || balance.Available != 12.75 || balance.Voucher != 10 || balance.Cash != 2.75 {
		t.Fatalf("DecodeBalance = %+v, %v", balance, err)
	}
	owed, err := moonshotapi.DecodeBalance([]byte(`{"code":0,"status":true,"data":{"available_balance":-1,"voucher_balance":0,"cash_balance":-1}}`))
	if err != nil || owed.Cash != -1 {
		t.Fatalf("owed = %+v, %v", owed, err)
	}
}

func TestAChangedBalanceIsRefused(t *testing.T) {
	for _, body := range []string{
		`{}`, `[]`,
		`{"code":1,"status":true,"data":{"available_balance":1,"voucher_balance":0,"cash_balance":1}}`,
		`{"code":0,"status":false,"data":{"available_balance":1,"voucher_balance":0,"cash_balance":1}}`,
		`{"code":0,"status":true,"data":{"available_balance":1,"cash_balance":1}}`,
		`{"code":0,"status":true,"data":{"available_balance":1,"voucher_balance":-1,"cash_balance":1}}`,
		`{"code":0,"status":true,"data":{"available_balance":1e12,"voucher_balance":0,"cash_balance":1}}`,
	} {
		if _, err := moonshotapi.DecodeBalance([]byte(body)); err == nil {
			t.Fatalf("DecodeBalance(%s) accepted", body)
		}
	}
}

// FuzzDecodeBalance checks that no answer panics the decoder.
func FuzzDecodeBalance(f *testing.F) {
	f.Add(read(f, "balance.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		if balance, err := moonshotapi.DecodeBalance(body); err == nil && balance.Voucher < 0 {
			t.Fatalf("negative voucher from %q", body)
		}
	})
}
