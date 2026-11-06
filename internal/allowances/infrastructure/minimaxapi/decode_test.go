package minimaxapi_test

import (
	"math"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/minimaxapi"
)

func remains(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/remains.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheCountsHoldWhatRemains(t *testing.T) {
	meters, err := minimaxapi.DecodeRemains(remains(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(meters) != 1 {
		t.Fatalf("a week without quota adds no meter: %+v", meters)
	}
	meter := meters[0]
	// 1349 of 1500 remain: 10.07 % used.
	if meter.Window() != domain.FiveHourWindow() || math.Abs(meter.Used().Float64()-10.0667) > 0.001 || meter.Bucket() != "minimax-m_" {
		t.Fatalf("meter = %+v", meter)
	}
	if resetsAt, _ := meter.ResetsAt(); !resetsAt.Equal(time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("reset = %v", resetsAt)
	}
}

func TestExplicitRemainingPercentagesWin(t *testing.T) {
	body := `{"model_remains":[{"model_name":"m","start_time":1,"end_time":14400001,"current_interval_total_count":0,"current_interval_remaining_percent":93,"current_weekly_remaining_percent":60,"weekly_end_time":1794268800000}],"base_resp":{"status_code":0}}`
	meters, err := minimaxapi.DecodeRemains([]byte(body))
	if err != nil || len(meters) != 2 {
		t.Fatalf("DecodeRemains = %+v, %v", meters, err)
	}
	if meters[0].Used() != 7 || meters[0].Window().Kind() != domain.WindowOther || meters[0].Window().Minutes() != 240 {
		t.Fatalf("a four-hour interval = %+v", meters[0])
	}
	if meters[1].Used() != 40 || meters[1].Window() != domain.WeeklyWindow() {
		t.Fatalf("week = %+v", meters[1])
	}
}

func TestFailuresInBaseResp(t *testing.T) {
	for _, body := range []string{
		`{"model_remains":[]}`,
		`{"base_resp":{"status_code":1004,"status_msg":"invalid api key"}}`,
		`{"base_resp":{"status_code":1000}}`,
		`{"model_remains":[{"model_name":"m","start_time":1,"end_time":14400001,"current_interval_remaining_percent":-50000}],"base_resp":{"status_code":0}}`,
	} {
		if _, err := minimaxapi.DecodeRemains([]byte(body)); err == nil {
			t.Fatalf("DecodeRemains(%s) accepted", body)
		}
	}
	if meters, err := minimaxapi.DecodeRemains([]byte(`{"model_remains":[],"base_resp":{"status_code":0}}`)); err != nil || len(meters) != 0 {
		t.Fatalf("an empty plan = %v", err)
	}
}

// FuzzDecodeRemains checks that no answer panics the decoder or yields a
// negative percentage.
func FuzzDecodeRemains(f *testing.F) {
	f.Add(remains(f))
	f.Add([]byte(`{"model_remains":[{"start_time":9223372036854775807,"end_time":-1,"current_interval_total_count":1e308}],"base_resp":{"status_code":0}}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		meters, err := minimaxapi.DecodeRemains(body)
		if err != nil {
			return
		}
		for _, meter := range meters {
			if meter.Used() < 0 {
				t.Fatalf("negative usage from %q", body)
			}
		}
	})
}
