package openrouterbenchmarks_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/openrouterbenchmarks"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOnlyMeasuredArtificialAnalysisRowsCount(t *testing.T) {
	scores, err := openrouterbenchmarks.Decode(read(t, "benchmarks.json"))
	if err != nil || len(scores) != 2 || scores[0].Name != "Coder Max (High)" || scores[1].Name != "example/coder-max-20260816" || scores[1].Index != 55.5 {
		t.Fatalf("Decode = %+v, %v", scores, err)
	}
	indexer := domain.Ranking{Scores: scores}.Indexer()
	if index, ok := indexer.IndexOf(domain.Offering{Model: "example/coder-max", Name: "Coder Max"}); !ok || index != 55.5 {
		t.Fatalf("a models.dev model = %v, %v", index, ok)
	}
	for _, body := range []string{`[]`, `{}`, `{"data": {}}`, `{"data": [{"source": "artificial-analysis", "display_name": "X", "intelligence_index": -3}]}`} {
		if _, err := openrouterbenchmarks.Decode([]byte(body)); err == nil {
			t.Fatalf("Decode(%s) accepted", body)
		}
	}
}

// FuzzDecode checks that no answer panics the decoder.
func FuzzDecode(f *testing.F) {
	f.Add(read(f, "benchmarks.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		scores, err := openrouterbenchmarks.Decode(body)
		if err != nil {
			return
		}
		for _, score := range scores {
			if score.Index < 0 || score.Name == "" {
				t.Fatalf("score %+v from %q", score, body)
			}
		}
	})
}
