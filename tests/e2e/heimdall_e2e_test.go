//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The production binary reads a fake Heimdall on loopback: plain HTTP is
// allowed there, and only there (ADR 0013).
func TestHeimdallIsReadOnLoopback(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	plans := fixture(t, "heimdallplans/testdata/plans.json")
	var mu sync.Mutex
	var authorization string
	heimdall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plans" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		authorization = r.Header.Get("Authorization")
		mu.Unlock()
		w.Write([]byte(plans))
	}))
	t.Cleanup(heimdall.Close)
	port := heimdall.Listener.Addr().(*net.TCPAddr).Port

	tr := newTree(t, fmt.Sprintf(`{"version": 1, "subscriptions": [
	  {"id": "gemini", "source": "heimdall", "provider": "google", "port": %d, "key": "heimdall"},
	  {"id": "groq", "source": "heimdall", "provider": "groq", "port": %d, "key": "heimdall"}
	]}`, port, port), nil)
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatalf("init = %d %s", code, stderr)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "synthetic-heimdall-admin-token\n", phrase, "keys", "set", "heimdall"); code != 0 {
		t.Fatalf("keys set = %d %s", code, stderr)
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "collect"); code != 0 {
		t.Fatalf("collect = %d %s", code, stderr)
	}
	mu.Lock()
	if authorization != "Bearer synthetic-heimdall-admin-token" {
		t.Fatalf("Heimdall received %q", authorization)
	}
	mu.Unlock()

	_, out, _ := tr.run(bin, "status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []struct {
				ID       string `json:"id"`
				Outcome  string `json:"outcome"`
				Fidelity string `json:"fidelity"`
				Meters   []struct {
					Bucket      *string `json:"bucket"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"meters"`
				Hint *string `json:"hint"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	gemini, groq := doc.Data.Subscriptions[0], doc.Data.Subscriptions[1]
	if gemini.Outcome != "OK" || gemini.Fidelity != "OBSERVED" || len(gemini.Meters) != 2 || gemini.Meters[1].UsedPercent != 100 ||
		gemini.Hint == nil || !strings.Contains(*gemini.Hint, "quota exhausted") {
		t.Fatalf("gemini = %+v", gemini)
	}
	if groq.Outcome != "OK" || len(groq.Meters) != 1 || groq.Meters[0].UsedPercent != 10 {
		t.Fatalf("groq = %+v", groq)
	}
	if strings.Contains(out, "synthetic-heimdall-admin-token") {
		t.Fatal("the admin token leaked into the status")
	}
}
