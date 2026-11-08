package cursoradminapi_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cursoradminapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheMemberIsFoundByEmail(t *testing.T) {
	spend, err := cursoradminapi.DecodeSpend(read(t, "spend.json"), "member@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if spend.OnDemand != 12.505 || spend.Overall != 42.505 || spend.Limit != 50 || !spend.CycleStart.Equal(time.UnixMilli(1793577600000).UTC()) {
		t.Fatalf("spend = %+v", spend)
	}
	other, err := cursoradminapi.DecodeSpend(read(t, "spend.json"), "someone.else@example.com")
	if err != nil || other.Limit != 0 || other.Overall != 12 {
		t.Fatalf("a member without a limit = %+v, %v", other, err)
	}
	if _, err := cursoradminapi.DecodeSpend(read(t, "spend.json"), "nobody@example.com"); !errors.Is(err, cursoradminapi.ErrNoMember) {
		t.Fatalf("an absent member = %v", err)
	}
}

func TestAChangedAnswerIsRefused(t *testing.T) {
	for _, body := range []string{
		`{}`, `[]`, `{"teamMemberSpend":{}}`,
		`{"teamMemberSpend":[{"email":"a@b.c","spendCents":1}]}`,
		`{"teamMemberSpend":[{"email":"a@b.c","spendCents":-1,"overallSpendCents":1}]}`,
		`{"teamMemberSpend":[{"email":"a@b.c","spendCents":1,"overallSpendCents":1}],"subscriptionCycleStart":1793577600}`,
	} {
		if _, err := cursoradminapi.DecodeSpend([]byte(body), "a@b.c"); err == nil {
			t.Fatalf("DecodeSpend(%s) accepted", body)
		}
	}
}

func TestTheCycleRenewsMonthly(t *testing.T) {
	start := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	if next := cursoradminapi.NextCycle(start, time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)); !next.Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("next cycle = %s", next)
	}
	if !cursoradminapi.NextCycle(time.Time{}, start).IsZero() {
		t.Fatal("an unknown start has no next cycle")
	}
}

// FuzzDecodeSpend checks that no answer panics the decoder.
func FuzzDecodeSpend(f *testing.F) {
	f.Add(read(f, "spend.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		if spend, err := cursoradminapi.DecodeSpend(body, "member@example.com"); err == nil && (spend.OnDemand < 0 || spend.Limit < 0) {
			t.Fatalf("negative figures from %q", body)
		}
	})
}
