package opencodezen_test

import (
	"os"
	"strings"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodezen"
)

func fixture(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/stats-models.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOnlyTheZenModelsAreCounted(t *testing.T) {
	spent, err := opencodezen.DecodeSpend(fixture(t))
	if err != nil || spent.String() != "4.75" {
		t.Fatalf("DecodeSpend = %s, %v", spent, err)
	}
}

func TestNoUsageYetIsZero(t *testing.T) {
	for _, output := range []string{`{"models": []}`, `{"models": [{"model": {"providerID": "anthropic"}, "cost": 1}]}`} {
		spent, err := opencodezen.DecodeSpend([]byte(output))
		if err != nil || spent.String() != "0" {
			t.Fatalf("DecodeSpend(%s) = %s, %v", output, spent, err)
		}
	}
}

func TestUnexpectedOutputIsRejected(t *testing.T) {
	for _, output := range []string{
		``,
		`not json`,
		`{"totals": {"cost": 3}}`,
		`{"models": [{"model": {"providerID": "opencode"}}]}`,
		`{"models": [{"model": {"providerID": "opencode"}, "cost": "free"}]}`,
		`{"models": [{"model": {"providerID": "opencode"}, "cost": -1}]}`,
		`{"models": [` + strings.Repeat(`{"model": {"providerID": "opencode"}, "cost": 1000000000000},`, 10) + `{}]}`,
	} {
		if _, err := opencodezen.DecodeSpend([]byte(output)); err == nil {
			t.Fatalf("DecodeSpend(%.60s) accepted", output)
		}
	}
}

// FuzzDecodeSpend checks that no output panics the decoder or yields a
// negative spend.
func FuzzDecodeSpend(f *testing.F) {
	f.Add(fixture(f))
	f.Add([]byte(`{"models": [{"model": {"providerID": "opencode"}, "cost": 1e-9}]}`))
	f.Add([]byte(`[[{"model": {"providerID": "opencode"}, "cost": 2}]]`))
	f.Fuzz(func(t *testing.T, output []byte) {
		spent, err := opencodezen.DecodeSpend(output)
		if err == nil && spent.IsNegative() {
			t.Fatalf("negative spend from %q", output)
		}
	})
}
