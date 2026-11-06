package commandcodeapi_test

import (
	"math"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/commandcodeapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCreditsAndWindows(t *testing.T) {
	credits, err := commandcodeapi.DecodeCredits(read(t, "credits.json"))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(credits.Remaining-47.7163) > 1e-9 || len(credits.Meters) != 2 {
		t.Fatalf("credits = %+v", credits)
	}
	five, week := credits.Meters[0], credits.Meters[1]
	if five.Window() != domain.FiveHourWindow() || math.Abs(five.Used().Float64()-0.37/14*100) > 1e-9 || week.Window() != domain.WeeklyWindow() {
		t.Fatalf("meters = %+v", credits.Meters)
	}
	spent, err := commandcodeapi.DecodeSummary(read(t, "summary.json"))
	if err != nil || spent != 22.2592 {
		t.Fatalf("DecodeSummary = %v, %v", spent, err)
	}
	subscription, err := commandcodeapi.DecodeSubscriptions(read(t, "subscriptions.json"))
	if err != nil || subscription.Plan != "individual-pro" || !subscription.PeriodEnd.Equal(time.Date(2026, 11, 3, 1, 26, 37, 0, time.UTC)) {
		t.Fatalf("DecodeSubscriptions = %+v, %v", subscription, err)
	}
}

func TestUnexpectedBillingIsRejected(t *testing.T) {
	if _, err := commandcodeapi.DecodeCredits([]byte(`{"credits":{"monthlyCredits":1},"windowLimits":{"fiveHour":{"used":-50,"cap":1}}}`)); err != nil {
		t.Fatalf("negative usage is clamped: %v", err)
	}
	if _, err := commandcodeapi.DecodeCredits([]byte(`{"credits":{"monthlyCredits":1},"windowLimits":{"fiveHour":{"used":1e300,"cap":1e-300}}}`)); err == nil {
		t.Fatal("an absurd percentage is rejected")
	}
	for _, body := range []string{`[]`, `{"totalCost":-1}`, `{}`} {
		if _, err := commandcodeapi.DecodeSummary([]byte(body)); err == nil {
			t.Fatalf("DecodeSummary(%s) accepted", body)
		}
	}
	for _, body := range []string{`[]`, `{"success":false}`} {
		if _, err := commandcodeapi.DecodeSubscriptions([]byte(body)); err == nil {
			t.Fatalf("DecodeSubscriptions(%s) accepted", body)
		}
	}
}

// FuzzDecodeCredits checks that no answer panics the decoder or yields a
// negative percentage.
func FuzzDecodeCredits(f *testing.F) {
	f.Add(read(f, "credits.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		credits, err := commandcodeapi.DecodeCredits(body)
		if err != nil {
			return
		}
		for _, meter := range credits.Meters {
			if meter.Used() < 0 {
				t.Fatalf("negative usage from %q", body)
			}
		}
	})
}
