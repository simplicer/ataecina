package publishedoffers_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/infrastructure/publishedoffers"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheOffersFileIsRead(t *testing.T) {
	offers, err := publishedoffers.Decode(read(t, "offers.csv"))
	if err != nil || len(offers) != 2 {
		t.Fatalf("Decode = %+v, %v", offers, err)
	}
	lite, pro := offers[0], offers[1]
	if !lite.Sponsored || lite.Price != 5 || lite.Currency != "USD" || lite.Period != "first month" ||
		lite.URL != "https://example.com/plans?ref=ataecina" || !lite.Ends.Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("lite = %+v", lite)
	}
	if pro.Sponsored || pro.Title != "bYearly/b plan" || !pro.Ends.IsZero() {
		t.Fatalf("pro = %+v", pro)
	}
	if empty, err := publishedoffers.Decode([]byte(strings.Join(publishedoffers.Header, ",") + "\n")); err != nil || len(empty) != 0 {
		t.Fatalf("no offer = %+v, %v", empty, err)
	}
}

// The file the repository publishes must always be readable: a mistake in
// it would hide every offer.
func TestThePublishedFileIsValid(t *testing.T) {
	data, err := os.ReadFile("../../../../data/offers.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishedoffers.Decode(data); err != nil {
		t.Fatalf("data/offers.csv: %v", err)
	}
}

func TestABadOffersFileIsRefused(t *testing.T) {
	header := strings.Join(publishedoffers.Header, ",") + "\n"
	for name, body := range map[string]string{
		"empty":          "",
		"other header":   "id,provider\n",
		"short line":     header + "a,b,c\n",
		"no plan":        header + "a,Example,,t,1,USD,month,https://example.com,no,\n",
		"plain http":     header + "a,Example,P,t,1,USD,month,http://example.com,no,\n",
		"script link":    header + "a,Example,P,t,1,USD,month,javascript:alert(1),no,\n",
		"bad price":      header + "a,Example,P,t,cheap,USD,month,https://example.com,no,\n",
		"bad end":        header + "a,Example,P,t,1,USD,month,https://example.com,no,soon\n",
		"credentials":    header + "a,Example,P,t,1,USD,month,https://me:pw@example.com,no,\n",
		"unclosed quote": header + "a,\"Example,P,t,1,USD,month,https://example.com,no,\n",
	} {
		if _, err := publishedoffers.Decode([]byte(body)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	many := header + strings.Repeat("a,Example,P,t,1,USD,month,https://example.com,no,\n", 101)
	if _, err := publishedoffers.Decode([]byte(many)); err == nil {
		t.Fatal("more than 100 offers accepted")
	}
}

// FuzzDecode checks that no file panics the reader or lets a non-https
// link through.
func FuzzDecode(f *testing.F) {
	f.Add(read(f, "offers.csv"))
	f.Fuzz(func(t *testing.T, body []byte) {
		offers, err := publishedoffers.Decode(body)
		if err != nil {
			return
		}
		for _, offer := range offers {
			if !strings.HasPrefix(offer.URL, "https://") {
				t.Fatalf("link %q from %q", offer.URL, body)
			}
		}
	})
}
