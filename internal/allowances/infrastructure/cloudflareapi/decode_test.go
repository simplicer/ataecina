package cloudflareapi_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/cloudflareapi"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTodaysNeuronsAreSummed(t *testing.T) {
	neurons, err := cloudflareapi.DecodeNeurons(read(t, "neurons.json"))
	if err != nil || neurons != 3350.5 {
		t.Fatalf("DecodeNeurons = %v, %v", neurons, err)
	}
	quiet, err := cloudflareapi.DecodeNeurons([]byte(`{"data":{"viewer":{"accounts":[{"aiInferenceAdaptiveGroups":[]}]}}}`))
	if err != nil || quiet != 0 {
		t.Fatalf("a quiet day = %v, %v", quiet, err)
	}
	if _, err := cloudflareapi.DecodeNeurons([]byte(`{"data":{"viewer":{"accounts":[]}}}`)); !errors.Is(err, cloudflareapi.ErrNoAccount) {
		t.Fatalf("no account = %v", err)
	}
	var queryError cloudflareapi.QueryError
	if _, err := cloudflareapi.DecodeNeurons([]byte(`{"data":null,"errors":[{"message":"not authorized for that account"}]}`)); !errors.As(err, &queryError) {
		t.Fatalf("GraphQL error = %v", err)
	}
	for _, body := range []string{`{}`, `{"data":{}}`, `{"data":{"viewer":{"accounts":[{"aiInferenceAdaptiveGroups":[{"sum":{}}]}]}}}`,
		`{"data":{"viewer":{"accounts":[{"aiInferenceAdaptiveGroups":[{"sum":{"totalNeurons":-1}}]}]}}}`, `[]`} {
		if _, err := cloudflareapi.DecodeNeurons([]byte(body)); err == nil {
			t.Fatalf("DecodeNeurons(%s) accepted", body)
		}
	}
}

func TestTheQueryIsFixedAndOnlyTheVariablesChange(t *testing.T) {
	document, err := cloudflareapi.Request("0123456789abcdef0123456789abcdef", time.Date(2026, 11, 6, 23, 59, 0, 0, time.FixedZone("CET", 3600)))
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Query     string            `json:"query"`
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(document, &request); err != nil {
		t.Fatal(err)
	}
	if request.Variables["accountTag"] != "0123456789abcdef0123456789abcdef" || request.Variables["date"] != "2026-11-06" {
		t.Fatalf("variables = %v", request.Variables)
	}
	if strings.Contains(request.Query, "0123456789abcdef") || !strings.Contains(request.Query, "date_geq: $date") || strings.Contains(request.Query, "mutation") {
		t.Fatalf("query = %s", request.Query)
	}
}

// FuzzDecodeNeurons checks that no answer panics the decoder.
func FuzzDecodeNeurons(f *testing.F) {
	f.Add(read(f, "neurons.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		neurons, err := cloudflareapi.DecodeNeurons(body)
		if err == nil && neurons < 0 {
			t.Fatalf("negative neurons from %q", body)
		}
	})
}
