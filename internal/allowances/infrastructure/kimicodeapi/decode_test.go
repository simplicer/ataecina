package kimicodeapi_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/kimicodeapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheFiveHourAndWeeklyWindowsAreRead(t *testing.T) {
	usages, err := kimicodeapi.DecodeUsages(read(t, "usages.json"))
	if err != nil {
		t.Fatal(err)
	}
	if usages.Level != "INTERMEDIATE" || len(usages.Limits) != 2 {
		t.Fatalf("usages = %+v", usages)
	}
	five, week := usages.Limits[0], usages.Limits[1]
	if five.Window != domain.FiveHourWindow() || five.Used != 12 || !five.ResetsAt.Equal(time.Date(2026, 11, 6, 22, 10, 0, 0, time.UTC)) {
		t.Fatalf("five hours = %+v", five)
	}
	if week.Window != domain.WeeklyWindow() || week.Used != 36 {
		t.Fatalf("week = %+v", week)
	}
}

func TestNumbersAndUsedAreAccepted(t *testing.T) {
	usages, err := kimicodeapi.DecodeUsages([]byte(`{"limits":[{"window":{"duration":5,"timeUnit":"TIME_UNIT_HOUR"},"detail":{"limit":200,"used":50}}]}`))
	if err != nil || len(usages.Limits) != 1 || usages.Limits[0].Used != 25 || usages.Limits[0].Window != domain.FiveHourWindow() || !usages.Limits[0].ResetsAt.IsZero() {
		t.Fatalf("usages = %+v, %v", usages, err)
	}
	weekly, err := kimicodeapi.DecodeUsages([]byte(`{"limits":[{"window":{"duration":7,"timeUnit":"TIME_UNIT_DAY"},"detail":{"limit":"100","remaining":"40"}}],"usage":{"limit":"100","remaining":"90"}}`))
	if err != nil || len(weekly.Limits) != 1 || weekly.Limits[0].Used != 60 {
		t.Fatalf("a weekly limit read twice = %+v, %v", weekly, err)
	}
}

func TestAChangedAnswerIsRefused(t *testing.T) {
	for _, body := range []string{
		`{}`, `[]`, `{"limits":[]}`,
		`{"usage":{"limit":"0","remaining":"0"}}`,
		`{"usage":{"limit":"100"}}`,
		`{"usage":{"limit":"many","remaining":"1"}}`,
		`{"usage":{"limit":"100","remaining":"120"}}`,
		`{"usage":{"limit":"100","remaining":"10","resetTime":"tomorrow"}}`,
		`{"limits":[{"detail":{"limit":"100","remaining":"10"}}]}`,
		`{"limits":[{"window":{"duration":1,"timeUnit":"TIME_UNIT_YEAR"},"detail":{"limit":"100","remaining":"10"}}]}`,
	} {
		if _, err := kimicodeapi.DecodeUsages([]byte(body)); err == nil {
			t.Fatalf("DecodeUsages(%s) accepted", body)
		}
	}
}

// FuzzDecodeUsages checks that no answer panics the decoder.
func FuzzDecodeUsages(f *testing.F) {
	f.Add(read(f, "usages.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		usages, err := kimicodeapi.DecodeUsages(body)
		if err != nil {
			return
		}
		for _, limit := range usages.Limits {
			if limit.Used < 0 || limit.Used > 10000 {
				t.Fatalf("used %v from %q", limit.Used, body)
			}
		}
	})
}
