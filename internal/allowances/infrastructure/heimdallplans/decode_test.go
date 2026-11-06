package heimdallplans_test

import (
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/heimdallplans"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestThePlansOfTheContract(t *testing.T) {
	plans, err := heimdallplans.DecodePlans(read(t, "plans.json"))
	if err != nil || len(plans) != 3 {
		t.Fatalf("DecodePlans = %+v, %v", plans, err)
	}
	a, b := plans[0], plans[1]
	if a.ID != "google-free-a" || a.Provider != "google" || *a.UsedRatio != 0.62 || a.Unavailable != "" ||
		!a.NextResetAt.Equal(time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("a = %+v", a)
	}
	if b.UsedRatio != nil || b.Unavailable != "quota_exhausted" {
		t.Fatalf("b = %+v", b)
	}
	older, err := heimdallplans.DecodePlans(read(t, "plans-without-usage.json"))
	if err != nil || len(older) != 1 || older[0].UsedRatio != nil || older[0].NextResetAt.IsZero() {
		t.Fatalf("a Heimdall without usage = %+v, %v", older, err)
	}
	odd, err := heimdallplans.DecodePlans([]byte(`{"data":[{"id":"p","provider":"x","available":false,"unavailable_reason":"<script>"}]}`))
	if err != nil || odd[0].Unavailable != "unavailable" {
		t.Fatalf("an unknown reason is not echoed: %+v, %v", odd, err)
	}
	for _, body := range []string{`{}`, `{"data":[{"provider":"x"}]}`, `{"data":[{"id":"p","provider":"x","window_used_ratio":-1}]}`,
		`{"data":[{"id":"p","provider":"x","next_reset_at":"tomorrow"}]}`, `[]`} {
		if _, err := heimdallplans.DecodePlans([]byte(body)); err == nil {
			t.Fatalf("DecodePlans(%s) accepted", body)
		}
	}
}

// FuzzDecodePlans checks that no answer panics the decoder.
func FuzzDecodePlans(f *testing.F) {
	f.Add(read(f, "plans.json"))
	f.Add(read(f, "plans-without-usage.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		plans, err := heimdallplans.DecodePlans(body)
		if err != nil {
			return
		}
		for _, plan := range plans {
			if plan.ID == "" || plan.Provider == "" || (plan.UsedRatio != nil && *plan.UsedRatio < 0) {
				t.Fatalf("invalid plan %+v from %q", plan, body)
			}
		}
	})
}
