package statusjson_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/statusjson"
)

var taken = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func report(t *testing.T) domain.StatusReport {
	t.Helper()
	claude, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline", Plan: "PRO",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{
			domain.NewMeter("", domain.FiveHourWindow(), 23.5, taken.Add(140*time.Minute)),
			domain.NewMeter("", domain.WeeklyWindow(), 41.2, taken.Add(108*time.Hour)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	usd, _ := domain.NewCurrency("USD")
	left, _ := domain.ParseAmount("8.42")
	used, _ := domain.ParseAmount("11.58")
	remaining, spent := domain.NewMoney(left, usd), domain.NewMoney(used, usd)
	balance, _ := domain.NewBalance(&remaining, &spent)
	openrouter, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "openrouter", Provider: "openrouter", Source: "openrouter-api",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken.Add(2 * time.Second),
		Balance: &balance,
	})
	if err != nil {
		t.Fatal(err)
	}
	minimax, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "minimax", Provider: "minimax", Source: "minimax-api",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeBroken,
		ErrorClass: domain.ErrorContractChanged, Hint: "unexpected response shape",
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, []domain.Reading{claude, openrouter, minimax})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.StatusAt(taken.Add(time.Minute))
}

func TestTheDocumentFollowsTheContract(t *testing.T) {
	var out bytes.Buffer
	err := statusjson.Write(&out, report(t), statusjson.Meta{Version: "0.1.0-unstable", GeneratedAt: taken.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Subscriptions []map[string]any `json:"subscriptions"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data.Subscriptions) != 3 {
		t.Fatalf("%d subscriptions", len(doc.Data.Subscriptions))
	}
	claude := doc.Data.Subscriptions[0]
	want := map[string]any{
		"id": "claude", "provider": "anthropic", "plan": "PRO", "outcome": "OK", "fidelity": "REPORTED",
		"freshness": "FRESH", "source": "CLAUDE_STATUSLINE", "observed_at": "2026-11-06T19:40:00Z",
		"balance": nil, "credits": nil, "banked_resets": nil, "error_class": nil, "hint": nil,
	}
	for key, value := range want {
		if claude[key] != value {
			t.Fatalf("claude[%s] = %v, want %v", key, claude[key], value)
		}
	}
	meter := claude["meters"].([]any)[0].(map[string]any)
	if meter["window"] != "FIVE_HOURS" || meter["used_percent"] != 23.5 || meter["resets_at"] != "2026-11-06T22:00:00Z" || meter["estimated"] != false || meter["window_minutes"] != 300.0 {
		t.Fatalf("meter = %v", meter)
	}
	balance := doc.Data.Subscriptions[1]["balance"].(map[string]any)
	remaining := balance["remaining"].(map[string]any)
	if remaining["amount"] != 8.42 || remaining["currency"] != "USD" {
		t.Fatalf("balance = %v", balance)
	}
	if meters := doc.Data.Subscriptions[2]["meters"].([]any); len(meters) != 0 {
		t.Fatal("meters must be an empty list, never null")
	}
	if doc.Data.Subscriptions[2]["error_class"] != "CONTRACT_CHANGED" || doc.Data.Subscriptions[2]["freshness"] != "UNKNOWN" {
		t.Fatalf("minimax = %v", doc.Data.Subscriptions[2])
	}
	if doc.Meta["version"] != "0.1.0-unstable" || doc.Meta["generated_at"] != "2026-11-06T19:41:00Z" ||
		doc.Meta["snapshot_taken_at"] != "2026-11-06T19:40:00Z" || doc.Meta["worst_used_percent"] != 41.2 {
		t.Fatalf("meta = %v", doc.Meta)
	}
}

func TestPropertiesAreSnakeCaseAndEnumsUpperSnakeCase(t *testing.T) {
	var out bytes.Buffer
	_ = statusjson.Write(&out, report(t), statusjson.Meta{Version: "x", GeneratedAt: taken})
	keys := regexp.MustCompile(`"([^"]+)":`).FindAllStringSubmatch(out.String(), -1)
	snake := regexp.MustCompile(`^[a-z]+(_[a-z0-9]+)*$`)
	for _, key := range keys {
		if !snake.MatchString(key[1]) {
			t.Fatalf("property %q is not snake_case", key[1])
		}
	}
	for _, enum := range []string{`"outcome": "OK"`, `"fidelity": "REPORTED"`, `"source": "OPENROUTER_API"`, `"window": "WEEKLY"`} {
		if !strings.Contains(out.String(), enum) {
			t.Fatalf("missing %s", enum)
		}
	}
}

func TestTheModelCatalogueIsShownPerSubscription(t *testing.T) {
	free := make([]statusjson.Model, 60)
	for i := range free {
		free[i] = statusjson.Model{ID: "m", Name: "M"}
	}
	about := statusjson.Meta{Version: "x", GeneratedAt: taken, OfferingsRefreshedAt: taken.Add(-time.Hour), Offerings: map[string]statusjson.Offerings{
		"openrouter": {Catalogue: "openrouter", Name: "OpenRouter", Models: 374, FreeModels: free,
			NewModels: []statusjson.Model{{ID: "n", Name: "N", ReleasedOn: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)}}},
	}}
	var out bytes.Buffer
	if err := statusjson.Write(&out, report(t), about); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Subscriptions []struct {
				ID        string `json:"id"`
				Offerings *struct {
					Catalogue  string `json:"catalogue"`
					Models     int    `json:"models"`
					Free       int    `json:"free"`
					FreeModels []any  `json:"free_models"`
					NewModels  []struct {
						ReleasedOn string `json:"released_on"`
					} `json:"new_models"`
				} `json:"offerings"`
			} `json:"subscriptions"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	claude, openrouter := doc.Data.Subscriptions[0], doc.Data.Subscriptions[1]
	if claude.Offerings != nil || openrouter.Offerings == nil {
		t.Fatalf("offerings = %+v, %+v", claude.Offerings, openrouter.Offerings)
	}
	shown := openrouter.Offerings
	if shown.Catalogue != "openrouter" || shown.Models != 374 || shown.Free != 60 || len(shown.FreeModels) != 50 || shown.NewModels[0].ReleasedOn != "2026-10-30" {
		t.Fatalf("openrouter offerings = %+v", shown)
	}
	if doc.Meta["offerings_refreshed_at"] != "2026-11-06T18:40:00Z" {
		t.Fatalf("meta = %v", doc.Meta)
	}
}

func TestAnEmptyReportIsStillADocument(t *testing.T) {
	var out bytes.Buffer
	if err := statusjson.Write(&out, domain.Snapshot{}.StatusAt(taken), statusjson.Meta{Version: "x", GeneratedAt: taken}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"subscriptions": []`, `"snapshot_taken_at": null`, `"worst_used_percent": null`} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("missing %s in %s", fragment, out.String())
		}
	}
}

func TestProblemsAreRFC9457(t *testing.T) {
	var out bytes.Buffer
	if err := statusjson.WriteProblem(&out, statusjson.NewProblem("snapshot-unreadable", "The snapshot cannot be read", "mode 0644")); err != nil {
		t.Fatal(err)
	}
	var problem map[string]string
	if err := json.Unmarshal(out.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem["type"] != "urn:ataecina:problem:snapshot-unreadable" || problem["title"] == "" || problem["detail"] != "mode 0644" {
		t.Fatalf("problem = %v", problem)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestWriteFailuresAreReported(t *testing.T) {
	if err := statusjson.WriteProblem(brokenWriter{}, statusjson.NewProblem("x", "y", "")); err == nil {
		t.Fatal("a failed write must be reported")
	}
}

func TestTheRatePeriodIsShownWhereThePlanPublishesPeakHours(t *testing.T) {
	start, end := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	about := statusjson.Meta{Version: "x", GeneratedAt: taken, RatePeriods: map[string]domain.RatePeriod{
		"openrouter": {Current: domain.RateOffPeak, EndsAt: start, PeakStartsAt: start, PeakEndsAt: end, Benefit: domain.OffPeakHalfCredits},
	}}
	var out bytes.Buffer
	if err := statusjson.Write(&out, report(t), about); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Subscriptions []struct {
				RatePeriod *struct {
					Current  string `json:"current"`
					EndsAt   string `json:"ends_at"`
					NextPeak struct {
						StartsAt string `json:"starts_at"`
						EndsAt   string `json:"ends_at"`
					} `json:"next_peak"`
					OffPeakBenefit string `json:"off_peak_benefit"`
				} `json:"rate_period"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	claude, openrouter := doc.Data.Subscriptions[0], doc.Data.Subscriptions[1]
	if claude.RatePeriod != nil {
		t.Fatalf("claude = %+v", claude.RatePeriod)
	}
	period := openrouter.RatePeriod
	if period == nil || period.Current != "OFF_PEAK" || period.EndsAt != "2026-11-09T06:00:00Z" ||
		period.NextPeak.StartsAt != "2026-11-09T06:00:00Z" || period.NextPeak.EndsAt != "2026-11-09T10:00:00Z" || period.OffPeakBenefit != "HALF_CREDITS" {
		t.Fatalf("rate period = %+v", period)
	}
	if !strings.Contains(out.String(), `"rate_period": null`) {
		t.Fatal("a plan without peak hours must say null")
	}
}
