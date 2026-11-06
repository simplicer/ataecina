package readingjson_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/readingjson"
)

var observed = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func fullReading(t *testing.T) domain.Reading {
	t.Helper()
	usd, _ := domain.NewCurrency("USD")
	left, _ := domain.ParseAmount("8.42")
	spentAmount, _ := domain.ParseAmount("11.58")
	remaining, spent := domain.NewMoney(left, usd), domain.NewMoney(spentAmount, usd)
	balance, _ := domain.NewBalance(&remaining, &spent)
	creditAmount, _ := domain.ParseAmount("12.5")
	credits := domain.NewCredits(creditAmount, false)
	banked, _ := domain.NewBankedResets(2)
	other, _ := domain.OtherWindow(90)
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "codex", Provider: "openai", Source: "codex-logs", Plan: domain.NewPlan("plus"),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: observed,
		Meters: []domain.Meter{
			domain.NewMeter("codex", domain.FiveHourWindow(), 23.5, observed.Add(2*time.Hour)),
			domain.NewMeter("codex", domain.WeeklyWindow(), 41.2, observed.Add(96*time.Hour)),
			domain.NewMeter("premium", domain.MonthlyWindow(), 130, time.Time{}),
			domain.NewMeter("", other, 5, time.Time{}),
		},
		Balance: &balance, Credits: &credits, BankedResets: &banked,
		ErrorClass: domain.ErrorUnavailable, Hint: "kept", RetryNotBefore: observed.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	return reading
}

func TestReadingsRoundTrip(t *testing.T) {
	original := fullReading(t)
	data, err := json.Marshal(readingjson.EncodeReading(original))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"observed_at":"2026-11-06T19:40:00Z"`,
		`"window":"FIVE_HOURS","window_minutes":300`,
		`"window":"MONTHLY","window_minutes":null`,
		`"remaining":{"amount":8.42,"currency":"USD"}`,
		`"credits":{"available":12.5,"unlimited":false}`,
		`"banked_resets":{"available":2}`,
		`"plan":"PLUS"`,
	} {
		if !strings.Contains(string(data), fragment) {
			t.Fatalf("missing %s in %s", fragment, data)
		}
	}
	var document readingjson.Reading
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	decoded, err := readingjson.DecodeReading(document)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(readingjson.EncodeReading(decoded), readingjson.EncodeReading(original)) {
		t.Fatalf("round trip changed the reading:\n%+v\n%+v", decoded, original)
	}
}

func TestAReadingWithoutFiguresUsesNulls(t *testing.T) {
	reading, _ := domain.NewReading(domain.ReadingFacts{
		Subscription: "zai", Provider: "zai", Source: "zai-api", Fidelity: domain.FidelityReported, Outcome: domain.OutcomeUnsupported,
	})
	data, _ := json.Marshal(readingjson.EncodeReading(reading))
	want := `{"subscription":"zai","provider":"zai","source":"zai-api","plan":null,"fidelity":"REPORTED","outcome":"UNSUPPORTED","observed_at":null,"meters":[],"balance":null,"credits":null,"banked_resets":null,"error_class":null,"hint":null}`
	if string(data) != want {
		t.Fatalf("got  %s\nwant %s", data, want)
	}
}

func TestDecodingRejectsInvalidDocuments(t *testing.T) {
	valid := readingjson.EncodeReading(fullReading(t))
	breakers := map[string]func(d *readingjson.Reading){
		"subscription": func(d *readingjson.Reading) { d.Subscription = "Bad" },
		"provider":     func(d *readingjson.Reading) { d.Provider = "" },
		"source":       func(d *readingjson.Reading) { d.Source = "x y" },
		"window":       func(d *readingjson.Reading) { d.Meters = []readingjson.Meter{{Window: "HOURLY"}} },
		"percent":      func(d *readingjson.Reading) { d.Meters = []readingjson.Meter{{Window: "MONTHLY", UsedPercent: -1}} },
		"amount": func(d *readingjson.Reading) {
			d.Balance = &readingjson.Balance{Spent: &readingjson.Money{Amount: "x", Currency: "USD"}}
		},
		"currency": func(d *readingjson.Reading) {
			d.Balance = &readingjson.Balance{Spent: &readingjson.Money{Amount: "1", Currency: "usd"}}
		},
		"balance": func(d *readingjson.Reading) { d.Balance = &readingjson.Balance{} },
		"credits": func(d *readingjson.Reading) { d.Credits = &readingjson.Credits{Available: "?"} },
		"banked":  func(d *readingjson.Reading) { d.BankedResets = &readingjson.BankedResets{Available: -1} },
		"outcome": func(d *readingjson.Reading) { d.Outcome = "FINE" },
	}
	for name, breakIt := range breakers {
		document := valid
		breakIt(&document)
		if _, err := readingjson.DecodeReading(document); err == nil {
			t.Fatalf("%s: DecodeReading accepted a broken document", name)
		}
	}
}

func TestInstantsAreStrictRFC3339(t *testing.T) {
	var instant readingjson.Instant
	if err := json.Unmarshal([]byte(`"2026-11-06T21:40:00+02:00"`), &instant); err != nil || !instant.Equal(observed) {
		t.Fatalf("Unmarshal = %v, %v", instant, err)
	}
	for _, bad := range []string{`"yesterday"`, `12`} {
		if err := json.Unmarshal([]byte(bad), &instant); err == nil {
			t.Fatalf("Unmarshal(%s) accepted", bad)
		}
	}
	if readingjson.InstantOf(time.Time{}, true) != nil || !readingjson.Moment(nil).IsZero() {
		t.Fatal("zero instants are null")
	}
}
