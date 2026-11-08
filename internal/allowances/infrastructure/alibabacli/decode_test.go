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
	usage, err := alibabacli.DecodeUsage(read(t, "token-plan.json"))
	if err != nil || len(usage.Windows) != 2 {
		t.Fatalf("DecodeUsage = %+v, %v", usage, err)
	}
	windows := usage.Windows
	if windows[0].Window != domain.FiveHourWindow() || windows[0].Fraction != 0.125 || !windows[0].ResetsAt.Equal(time.Date(2026, 11, 6, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("five hours = %+v", windows[0])
	}
	if windows[1].Window != domain.WeeklyWindow() || windows[1].Fraction != 0.7 {
		t.Fatalf("week = %+v", windows[1])
	}
	monthly, err := alibabacli.DecodeUsage([]byte(`{"per1MonthPercentage": 0.0139, "per1MonthResetTime": 1795795200000}`))
	if err != nil || len(monthly.Windows) != 1 || monthly.Windows[0].Window != domain.MonthlyWindow() {
		t.Fatalf("bl 2.2 monthly = %+v, %v", monthly, err)
	}
	daily, err := alibabacli.DecodeUsage([]byte(`{"per1DayPercentage": 0.5, "per3MonthPercentage": 0.1, "per2HourPercentage": 0.2}`))
	if err != nil || len(daily.Windows) != 2 || daily.Windows[0].Window.Minutes() != 120 || daily.Windows[1].Window != domain.DailyWindow() {
		t.Fatalf("windows found by name = %+v, %v", daily, err)
	}
	if _, err := alibabacli.DecodeUsage([]byte(`{"hasQuotaData": false}`)); !errors.Is(err, alibabacli.ErrNoQuota) {
		t.Fatalf("no quota = %v", err)
	}
	for _, body := range []string{`{}`, `{"hasQuotaData": true}`, `{"per5HourPercentage": -0.1}`, `{"per5HourPercentage": "high"}`,
		`{"per5HourPercentage": 0.1, "per5HourResetTime": 1793995200}`, `{"per5HourPercentage": 0.1, "per5HourResetTime": "soon"}`, `[]`} {
		if _, err := alibabacli.DecodeUsage([]byte(body)); err == nil {
			t.Fatalf("DecodeUsage(%s) accepted", body)
		}
	}
}

func TestTheCodingPlanGivesItsThreeWindows(t *testing.T) {
	usage, err := alibabacli.DecodeCodingPlan(read(t, "coding-plan.json"))
	if err != nil || usage.Plan != "pro" || len(usage.Windows) != 3 {
		t.Fatalf("DecodeCodingPlan = %+v, %v", usage, err)
	}
	five, week, month := usage.Windows[0], usage.Windows[1], usage.Windows[2]
	if five.Window != domain.FiveHourWindow() || five.Fraction != 0.38 || week.Fraction != 0.2 || month.Window != domain.BillingCycleWindow() {
		t.Fatalf("windows = %+v", usage.Windows)
	}
	if _, err := alibabacli.DecodeCodingPlan([]byte(`{}`)); !errors.Is(err, alibabacli.ErrNoQuota) {
		t.Fatalf("no valid subscription = %v", err)
	}
	for _, body := range []string{`[]`, `{"instanceType":"pro"}`, `{"per5Hour":{"usedQuota":1}}`, `{"per5Hour":{"percentage":-1}}`} {
		if _, err := alibabacli.DecodeCodingPlan([]byte(body)); err == nil {
			t.Fatalf("DecodeCodingPlan(%s) accepted", body)
		}
	}
}

// FuzzDecodeUsage checks that no output panics the decoder.
func FuzzDecodeUsage(f *testing.F) {
	f.Add(read(f, "token-plan.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		usage, err := alibabacli.DecodeUsage(body)
		if err == nil && len(usage.Windows) == 0 {
			t.Fatalf("no window from %q", body)
		}
	})
}

// FuzzDecodeCodingPlan checks that no output panics the decoder.
func FuzzDecodeCodingPlan(f *testing.F) {
	f.Add(read(f, "coding-plan.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		usage, err := alibabacli.DecodeCodingPlan(body)
		if err == nil && len(usage.Windows) == 0 {
			t.Fatalf("no window from %q", body)
		}
	})
}
