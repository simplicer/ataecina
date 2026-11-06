package claudestatusline_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/claudestatusline"
)

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOnlyTheRateLimitsAreRead(t *testing.T) {
	meters, err := claudestatusline.DecodeRateLimits(fixture(t, "statusline-pro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(meters) != 2 {
		t.Fatalf("%d meters", len(meters))
	}
	if meters[0].Window() != domain.FiveHourWindow() || meters[0].Used() != 23.5 {
		t.Fatalf("five hours = %+v", meters[0])
	}
	if resetsAt, _ := meters[0].ResetsAt(); !resetsAt.Equal(time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("reset = %v", resetsAt)
	}
	if meters[1].Window() != domain.WeeklyWindow() || meters[1].Used() != 41.2 {
		t.Fatalf("week = %+v", meters[1])
	}
}

func TestASpendLimitBehindAGatewayIsItsOwnBucket(t *testing.T) {
	meters, err := claudestatusline.DecodeRateLimits([]byte(`{"rate_limits":{"spend_limit":{"used_percentage":12,"resets_at":1794474000}}}`))
	if err != nil || len(meters) != 1 || meters[0].Bucket() != "spend_limit" || meters[0].Window() != domain.BillingCycleWindow() {
		t.Fatalf("spend limit = %+v, %v", meters, err)
	}
}

func TestNoRateLimitsYet(t *testing.T) {
	for _, input := range []string{`{"model":{"id":"x"}}`, `{"rate_limits":null}`, `{"rate_limits":{}}`} {
		meters, err := claudestatusline.DecodeRateLimits([]byte(input))
		if err != nil || len(meters) != 0 {
			t.Fatalf("DecodeRateLimits(%s) = %v, %v", input, meters, err)
		}
	}
	meters, err := claudestatusline.DecodeRateLimits([]byte(`{"rate_limits":{"five_hour":{"used_percentage":3}}}`))
	if err != nil || len(meters) != 1 {
		t.Fatalf("a window without reset = %v, %v", meters, err)
	}
	if _, known := meters[0].ResetsAt(); known {
		t.Fatal("the reset is unknown")
	}
}

func TestMalformedInputIsRejected(t *testing.T) {
	for _, input := range []string{
		``,
		`[]`,
		`{"rate_limits":{"five_hour":{"resets_at":1794474000}}}`,
		`{"rate_limits":{"five_hour":{"used_percentage":-1}}}`,
		`{"rate_limits":{"five_hour":{"used_percentage":"high"}}}`,
		`{"rate_limits":{"seven_day":{"used_percentage":5,"resets_at":1794474000000}}}`,
		`{"rate_limits":{"seven_day":{"used_percentage":5,"resets_at":12}}}`,
	} {
		if _, err := claudestatusline.DecodeRateLimits([]byte(input)); !errors.Is(err, claudestatusline.ErrMalformed) {
			t.Fatalf("DecodeRateLimits(%q) = %v", input, err)
		}
	}
}

// FuzzDecodeRateLimits checks that no status line input panics the
// decoder or yields an impossible meter.
func FuzzDecodeRateLimits(f *testing.F) {
	f.Add(fixture(f, "statusline-pro.json"))
	f.Add([]byte(`{"rate_limits":{"spend_limit":{"used_percentage":12,"resets_at":1794474000.5}}}`))
	f.Add([]byte(`{"rate_limits":{"five_hour":{"used_percentage":1e308,"resets_at":-1}}}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		meters, err := claudestatusline.DecodeRateLimits(input)
		if err != nil {
			return
		}
		for _, meter := range meters {
			if meter.Used() < 0 {
				t.Fatalf("negative usage from %q", input)
			}
		}
	})
}
