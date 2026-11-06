package orcarouterapi_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/orcarouterapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheAllowanceAndTheSpending(t *testing.T) {
	allowance, err := orcarouterapi.DecodeSubscription(read(t, "subscription.json"))
	if err != nil || allowance.HardLimitUSD != 20 || !allowance.ExpiresAt.Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("DecodeSubscription = %+v, %v", allowance, err)
	}
	if forever, err := orcarouterapi.DecodeSubscription([]byte(`{"hard_limit_usd": 5, "access_until": 0}`)); err != nil || !forever.ExpiresAt.IsZero() {
		t.Fatalf("no expiry = %+v, %v", forever, err)
	}
	spent, err := orcarouterapi.DecodeUsage(read(t, "usage.json"))
	if err != nil || spent != 11.58 {
		t.Fatalf("DecodeUsage = %v, %v", spent, err)
	}
	for _, body := range []string{`{}`, `{"hard_limit_usd": -1}`, `{"hard_limit_usd": 1, "access_until": 1798675200000}`, `[]`} {
		if _, err := orcarouterapi.DecodeSubscription([]byte(body)); err == nil {
			t.Fatalf("DecodeSubscription(%s) accepted", body)
		}
	}
	for _, body := range []string{`{}`, `{"total_usage": -5}`, `[]`} {
		if _, err := orcarouterapi.DecodeUsage([]byte(body)); err == nil {
			t.Fatalf("DecodeUsage(%s) accepted", body)
		}
	}
}

// FuzzDecodeSubscription checks that no answer panics the decoder.
func FuzzDecodeSubscription(f *testing.F) {
	f.Add(read(f, "subscription.json"))
	f.Add(read(f, "usage.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		allowance, err := orcarouterapi.DecodeSubscription(body)
		if err == nil && allowance.HardLimitUSD < 0 {
			t.Fatalf("negative allowance from %q", body)
		}
		spent, err := orcarouterapi.DecodeUsage(body)
		if err == nil && spent < 0 {
			t.Fatalf("negative spending from %q", body)
		}
	})
}
