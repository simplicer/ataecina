package opencodeconsole_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodeconsole"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheGoWindowsAndTheBudgetsAreRead(t *testing.T) {
	limits, err := opencodeconsole.DecodeLimits(read(t, "limits.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(limits.Go) != 3 || limits.Go[0].Window != domain.FiveHourWindow() || limits.Go[1].Window != domain.WeeklyWindow() || limits.Go[2].Window != domain.MonthlyWindow() {
		t.Fatalf("go = %+v", limits.Go)
	}
	if limits.Go[1].Percent != 40.5 || !limits.Go[0].ResetsAt.Equal(time.UnixMilli(1794141000000)) {
		t.Fatalf("go = %+v", limits.Go)
	}
	if len(limits.Budgets) != 1 || limits.Budgets[0].Limit != 20 || limits.Budgets[0].Spent != 5 {
		t.Fatalf("budgets = %+v", limits.Budgets)
	}
}

func TestASignedOutOpenCodeIsTold(t *testing.T) {
	if _, err := opencodeconsole.DecodeLimits(read(t, "signed-out.json")); !errors.Is(err, opencodeconsole.ErrSignedOut) {
		t.Fatalf("signed out = %v", err)
	}
	budgetsOnly, err := opencodeconsole.DecodeLimits([]byte(`{"data":{"workspace":{"id":"w","usageURL":"u"},"budgets":[]}}`))
	if err != nil || len(budgetsOnly.Go) != 0 || len(budgetsOnly.Budgets) != 0 {
		t.Fatalf("no go, no budget = %+v, %v", budgetsOnly, err)
	}
}

func TestAChangedAnswerIsRefused(t *testing.T) {
	window := `{"percent":10,"limited":false,"resetsAt":1794141000000}`
	for _, body := range []string{
		`{}`, `[]`, `{"data":{}}`, `{"data":{"budgets":{}}}`,
		`{"data":{"budgets":[],"go":{"rolling":` + window + `}}}`,
		`{"data":{"budgets":[],"go":{"rolling":{"percent":101,"limited":false,"resetsAt":0},"weekly":` + window + `,"monthly":` + window + `}}}`,
		`{"data":{"budgets":[],"go":{"rolling":{"percent":1,"limited":false,"resetsAt":1794141000},"weekly":` + window + `,"monthly":` + window + `}}}`,
		`{"data":{"budgets":[{"limit":10,"spent":-1,"resetsAt":0}]}}`,
		`{"data":{"budgets":[{"limit":10,"resetsAt":0}]}}`,
	} {
		if _, err := opencodeconsole.DecodeLimits([]byte(body)); err == nil {
			t.Fatalf("DecodeLimits(%s) accepted", body)
		}
	}
}

// FuzzDecodeLimits checks that no output panics the decoder.
func FuzzDecodeLimits(f *testing.F) {
	f.Add(read(f, "limits.json"))
	f.Add(read(f, "signed-out.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		limits, err := opencodeconsole.DecodeLimits(body)
		if err != nil {
			return
		}
		for _, window := range limits.Go {
			if window.Percent < 0 || window.Percent > 100 {
				t.Fatalf("percent %v from %q", window.Percent, body)
			}
		}
	})
}
