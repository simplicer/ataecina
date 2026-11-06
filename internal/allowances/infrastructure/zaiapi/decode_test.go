package zaiapi_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/zaiapi"
)

func quota(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/quota.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCreditAndTokenLimitsBecomeWindows(t *testing.T) {
	plan, err := zaiapi.DecodeQuota(quota(t))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Level != "lite" || len(plan.Meters) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	five, week := plan.Meters[0], plan.Meters[1]
	if five.Window() != domain.FiveHourWindow() || five.Used() != 12 || week.Window() != domain.WeeklyWindow() || week.Used() != 41 {
		t.Fatalf("meters = %+v", plan.Meters)
	}
	if resetsAt, _ := five.ResetsAt(); !resetsAt.Equal(time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("five-hour reset = %v", resetsAt)
	}
	older := `{"code":200,"success":true,"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":7}]}}`
	untouched, err := zaiapi.DecodeQuota([]byte(older))
	if err != nil || len(untouched.Meters) != 1 {
		t.Fatalf("TOKENS_LIMIT = %v", err)
	}
	if _, known := untouched.Meters[0].ResetsAt(); known {
		t.Fatal("an untouched window has no reset")
	}
}

func TestUnexpectedQuotasAreRejected(t *testing.T) {
	for _, body := range []string{
		`{"code":500,"success":false}`,
		`{"code":200,"success":true,"data":{"limits":[]}}`,
		`{"code":200,"success":true,"data":{"limits":[{"type":"CREDIT_LIMIT","unit":9,"number":1,"percentage":1}]}}`,
		`{"code":200,"success":true,"data":{"limits":[{"type":"CREDIT_LIMIT","unit":3,"number":5}]}}`,
		`{"code":200,"success":true,"data":{"limits":[{"type":"CREDIT_LIMIT","unit":3,"number":5,"percentage":-4}]}}`,
		`{"code":401,"success":false}`,
	} {
		if _, err := zaiapi.DecodeQuota([]byte(body)); err == nil {
			t.Fatalf("DecodeQuota(%s) accepted", body)
		}
	}
}

// FuzzDecodeQuota checks that no answer panics the decoder or yields a
// negative percentage.
func FuzzDecodeQuota(f *testing.F) {
	f.Add(quota(f))
	f.Add([]byte(`{"code":200,"success":true,"data":{"limits":[{"type":"CREDIT_LIMIT","unit":6,"number":999999,"percentage":1e9,"nextResetTime":-1}]}}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		plan, err := zaiapi.DecodeQuota(body)
		if err != nil {
			return
		}
		for _, meter := range plan.Meters {
			if meter.Used() < 0 || meter.Window().Minutes() <= 0 {
				t.Fatalf("impossible meter from %q", body)
			}
		}
	})
}
