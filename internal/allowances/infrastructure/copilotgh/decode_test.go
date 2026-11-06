package copilotgh_test

import (
	"os"
	"slices"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/copilotgh"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOnlyCopilotItemsCount(t *testing.T) {
	usage, err := copilotgh.DecodeReport(read(t, "report.json"))
	if err != nil || usage.Quantity != 165 || usage.NetAmount != 0.6 || usage.Items != 2 {
		t.Fatalf("DecodeReport = %+v, %v", usage, err)
	}
	empty, err := copilotgh.DecodeReport([]byte(`{"usageItems": []}`))
	if err != nil || empty.Items != 0 {
		t.Fatalf("an empty month = %+v, %v", empty, err)
	}
	for _, body := range []string{`{}`, `{"usageItems": [{"product": "Copilot"}]}`, `{"usageItems": [{"product": "copilot", "grossQuantity": -1, "netAmount": 0}]}`, `[]`} {
		if _, err := copilotgh.DecodeReport([]byte(body)); err == nil {
			t.Fatalf("DecodeReport(%s) accepted", body)
		}
	}
}

func TestTheArgvIsFixed(t *testing.T) {
	got := copilotgh.Arguments("octo-cat", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	want := []string{"api", "--method", "GET", "-H", "X-GitHub-Api-Version: 2026-03-10",
		"/users/octo-cat/settings/billing/premium_request/usage?year=2026&month=11"}
	if !slices.Equal(got, want) {
		t.Fatalf("Arguments = %q", got)
	}
}

// FuzzDecodeReport checks that no output panics the decoder.
func FuzzDecodeReport(f *testing.F) {
	f.Add(read(f, "report.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		usage, err := copilotgh.DecodeReport(body)
		if err == nil && (usage.Quantity < 0 || usage.NetAmount < 0) {
			t.Fatalf("negative usage from %q", body)
		}
	})
}
