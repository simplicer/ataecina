//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The intelligence ranking of the fake Artificial Analysis: one model that
// matches "Coder Max" of the models.dev fixture, in two variants.
const fakeRanking = `{"tier": "free", "intelligence_index_version": 5.0,
  "pagination": {"page": 1, "page_size": 200, "total_pages": 1, "has_more": false},
  "data": [
    {"name": "Coder Max (High)", "evaluations": {"artificial_analysis_intelligence_index": 55.5}},
    {"name": "Coder Max (Low)", "evaluations": {"artificial_analysis_intelligence_index": 41}}
  ]}`

// The published offers of the fake Codeberg: one sponsored, one not.
const fakeOffers = `id,provider,plan,title,price,currency,period,url,sponsored,ends
example-lite,Example AI,Coding Lite,Half price the first month,5,USD,first month,https://example.com/lite?ref=ataecina,yes,2099-12-31
example-pro,Example AI,Coding Pro,Yearly plan,90,USD,year,https://example.com/pro,no,
`

func TestUpdateBringsTheModelsAndTheirIntelligence(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "openrouter", "source": "openrouter-api"}]}`, nil)
	catalogue := fixture(t, "../../offerings/infrastructure/modelsdev/testdata/api.json")
	var mu sync.Mutex
	headers := map[string]http.Header{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers[r.URL.Path] = r.Header.Clone()
		mu.Unlock()
		switch r.URL.Path {
		case "/api.json":
			w.Write([]byte(catalogue))
		case "/api/v2/language/models/free":
			w.Write([]byte(fakeRanking))
		case "/simplicer/ataecina/raw/branch/main/data/offers.csv":
			w.Write([]byte(fakeOffers))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", server.URL)
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatalf("init = %d %s", code, stderr)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "aa-synthetic-key\n", phrase, "keys", "set", "artificial-analysis"); code != 0 {
		t.Fatalf("keys set = %d %s", code, stderr)
	}
	code, out, stderr := tr.runWithPassphrase(bin, "", phrase, "update")
	if code != 0 || !strings.Contains(out, "model catalogue: up to date for 1 subscriptions") || !strings.Contains(out, "intelligence index: 1 model, Artificial Analysis Intelligence Index v5") || !strings.Contains(out, "offers: 2 current") {
		t.Fatalf("update = %d %q %q", code, out, stderr)
	}
	mu.Lock()
	if got := headers["/api/v2/language/models/free"].Get("x-api-key"); got != "aa-synthetic-key" {
		t.Fatalf("the ranking received the key %q", got)
	}
	if got := headers["/api.json"].Get("x-api-key") + headers["/api.json"].Get("Authorization"); got != "" {
		t.Fatalf("the catalogue received a key: %q", got)
	}
	mu.Unlock()

	_, out, _ = tr.run(bin, "models", "--json", "openrouter")
	var listed struct {
		Data []struct {
			Subscription string `json:"subscription"`
			Models       []struct {
				ID           string   `json:"id"`
				Free         bool     `json:"free"`
				Intelligence *float64 `json:"intelligence"`
				Available    bool     `json:"available"`
			} `json:"models"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil || len(listed.Data) != 1 {
		t.Fatalf("models = %s %v", out, err)
	}
	first := listed.Data[0].Models[0]
	if first.ID != "example/coder-max" || first.Intelligence == nil || *first.Intelligence != 55.5 {
		t.Fatalf("best model = %+v", first)
	}
	// No snapshot yet: the openrouter reading is unknown, so only the free
	// models are available.
	for _, model := range listed.Data[0].Models {
		if model.Available != model.Free {
			t.Fatalf("availability = %+v", model)
		}
	}
	if listed.Meta["intelligence_index_version"] != "5" || listed.Meta["intelligence_url"] != "https://artificialanalysis.ai/" {
		t.Fatalf("meta = %v", listed.Meta)
	}
}

func TestOffersAreShownAndCanBeTurnedOff(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "zen", "source": "opencode-zen"}]}`, nil)
	asked := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/simplicer/ataecina/raw/branch/main/data/offers.csv" {
			asked++
			w.Write([]byte(fakeOffers))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", server.URL)
	tr.run(bin, "update")
	_, out, _ := tr.run(bin, "offers", "--json")
	var listed struct {
		Data []struct {
			ID        string  `json:"id"`
			URL       string  `json:"url"`
			Sponsored bool    `json:"sponsored"`
			Ends      *string `json:"ends"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil || len(listed.Data) != 2 || !listed.Data[0].Sponsored || listed.Data[1].Ends != nil {
		t.Fatalf("offers = %s %v", out, err)
	}
	off := newTree(t, `{"version": 1, "offers": false, "subscriptions": [{"id": "zen", "source": "opencode-zen"}]}`, nil)
	off.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", server.URL)
	before := asked
	_, out, _ = off.run(bin, "update")
	if asked != before || strings.Contains(out, "offers:") {
		t.Fatalf("offers fetched although turned off: %q", out)
	}
}

// OpenRouter's benchmarks: the Artificial Analysis row of "Coder Max".
const fakeBenchmarks = `{"data": [
  {"source": "artificial-analysis", "model_permaslug": "example/coder-max-20260816", "display_name": "Coder Max (High)", "intelligence_index": 52.5},
  {"source": "design-arena", "model_permaslug": "example/coder-max-20260816", "display_name": "Coder Max", "elo": 1200}
], "meta": {"version": "v1", "source": "artificial-analysis"}}`

func TestWithoutItsKeyTheIndexComesThroughOpenRouter(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "openrouter", "source": "openrouter-api"}]}`, nil)
	catalogue := fixture(t, "../../offerings/infrastructure/modelsdev/testdata/api.json")
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api.json":
			w.Write([]byte(catalogue))
		case "/api/v1/benchmarks":
			authorization = r.Header.Get("Authorization")
			w.Write([]byte(fakeBenchmarks))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", server.URL)
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatalf("init = %d %s", code, stderr)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "sk-or-synthetic\n", phrase, "keys", "set", "openrouter"); code != 0 {
		t.Fatalf("keys set = %d %s", code, stderr)
	}
	code, out, stderr := tr.runWithPassphrase(bin, "", phrase, "update")
	if code != 0 || !strings.Contains(out, "intelligence index: 1 model, Artificial Analysis Intelligence Index, via OpenRouter") {
		t.Fatalf("update = %d %q %q", code, out, stderr)
	}
	if authorization != "Bearer sk-or-synthetic" {
		t.Fatalf("OpenRouter received %q", authorization)
	}
	_, out, _ = tr.run(bin, "models", "--json", "openrouter")
	if !strings.Contains(out, `"intelligence": 52.5`) || !strings.Contains(out, `"intelligence_via": "OpenRouter"`) {
		t.Fatalf("models = %s", out)
	}
}
