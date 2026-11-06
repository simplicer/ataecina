package alibabacli_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/alibabacli"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEveryWindowBlReports(t *testing.T) {
	windows, err := alibabacli.DecodeUsage(read(t, "token-plan.json"))
	if err != nil || len(windows) != 2 {
		t.Fatalf("DecodeUsage = %+v, %v", windows, err)
	}
	if windows[0].Window != domain.FiveHourWindow() || windows[0].Fraction != 0.125 || !windows[0].ResetsAt.Equal(time.Date(2026, 11, 6, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("five hours = %+v", windows[0])
	}
	if windows[1].Window != domain.WeeklyWindow() || windows[1].Fraction != 0.7 {
		t.Fatalf("week = %+v", windows[1])
	}
	monthly, err := alibabacli.DecodeUsage([]byte(`{"per1MonthPercentage": 0.0139, "per1MonthResetTime": 1795795200000}`))
	if err != nil || len(monthly) != 1 || monthly[0].Window != domain.MonthlyWindow() {
		t.Fatalf("bl 2.2 monthly = %+v, %v", monthly, err)
	}
	if _, err := alibabacli.DecodeUsage([]byte(`{"hasQuotaData": false}`)); !errors.Is(err, alibabacli.ErrNoQuota) {
		t.Fatalf("no quota = %v", err)
	}
	for _, body := range []string{`{}`, `{"hasQuotaData": true}`, `{"per5HourPercentage": -0.1}`, `{"per5HourPercentage": 0.1, "per5HourResetTime": 1793995200}`, `[]`} {
		if _, err := alibabacli.DecodeUsage([]byte(body)); err == nil {
			t.Fatalf("DecodeUsage(%s) accepted", body)
		}
	}
}

// FuzzDecodeUsage checks that no output panics the decoder.
func FuzzDecodeUsage(f *testing.F) {
	f.Add(read(f, "token-plan.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		windows, err := alibabacli.DecodeUsage(body)
		if err == nil && len(windows) == 0 {
			t.Fatalf("no window from %q", body)
		}
	})
}
