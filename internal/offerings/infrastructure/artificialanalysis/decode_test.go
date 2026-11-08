package artificialanalysis_test

import (
	"os"
	"testing"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/artificialanalysis"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAPageGivesTheMeasuredModels(t *testing.T) {
	version, scores, more, err := artificialanalysis.DecodePage(read(t, "page1.json"))
	if err != nil || version != "5" || !more || len(scores) != 1 || scores[0].Name != "GLM-5.3 (Max)" || scores[0].Index != 61.2 {
		t.Fatalf("DecodePage = %q %+v %v %v", version, scores, more, err)
	}
	_, last, more, err := artificialanalysis.DecodePage(read(t, "page2.json"))
	if err != nil || more || last[0].Name != "Kimi K3 b" {
		t.Fatalf("last page = %+v %v %v", last, more, err)
	}
	for _, body := range []string{`[]`, `{}`, `{"data": {}}`, `{"data": [{"name": "X", "evaluations": {"artificial_analysis_intelligence_index": -1}}]}`} {
		if _, _, _, err := artificialanalysis.DecodePage([]byte(body)); err == nil {
			t.Fatalf("DecodePage(%s) accepted", body)
		}
	}
}

// FuzzDecodePage checks that no answer panics the decoder.
func FuzzDecodePage(f *testing.F) {
	f.Add(read(f, "page1.json"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, scores, _, err := artificialanalysis.DecodePage(body)
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
