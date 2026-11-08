//go:build e2e

package e2e

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeProviders serves the synthetic fixtures of every API adapter on one
// local server, and records which key each path received.
func fakeProviders(t *testing.T) (string, func() map[string]string) {
	t.Helper()
	routes := map[string]string{
		"/api/v1/key":                                "openrouterapi/testdata/key.json",
		"/api/v1/credits":                            "openrouterapi/testdata/credits.json",
		"/v1/credits":                                "vercelapi/testdata/credits.json",
		"/api/monitor/usage/quota/limit":             "zaiapi/testdata/quota.json",
		"/v1/token_plan/remains":                     "minimaxapi/testdata/remains.json",
		"/alpha/billing/credits":                     "commandcodeapi/testdata/credits.json",
		"/alpha/usage/summary":                       "commandcodeapi/testdata/summary.json",
		"/alpha/billing/subscriptions":               "commandcodeapi/testdata/subscriptions.json",
		"/api/balance":                               "ollamaapi/testdata/balance.json",
		"/v1/dashboard/billing/subscription":         "orcarouterapi/testdata/subscription.json",
		"/v1/dashboard/billing/usage":                "orcarouterapi/testdata/usage.json",
		"/client/v4/graphql":                         "cloudflareapi/testdata/neurons.json",
		"/v1/users/me/balance":                       "moonshotapi/testdata/balance.json",
		"/coding/v1/usages":                          "kimicodeapi/testdata/usages.json",
		"/v1/billing/teams/team-e2e/prepaid/balance": "xaiapi/testdata/balance.json",
		"/teams/spend":                               "cursoradminapi/testdata/spend.json",
		"/api.json":                                  "../../offerings/infrastructure/modelsdev/testdata/api.json",
	}
	bodies := make(map[string]string, len(routes))
	for path, name := range routes {
		bodies[path] = fixture(t, name)
	}
	var mu sync.Mutex
	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		seen[r.URL.Path] = r.Header.Get("Authorization") + " | " + r.Header.Get("User-Agent")
		mu.Unlock()
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL, func() map[string]string {
		mu.Lock()
		defer mu.Unlock()
		copied := map[string]string{}
		for path, value := range seen {
			copied[path] = value
		}
		return copied
	}
}

func TestEveryAPISourceWithSealedKeys(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [
	  {"id": "openrouter", "source": "openrouter-api"},
	  {"id": "vercel", "source": "vercel-api"},
	  {"id": "zai", "source": "zai-api"},
	  {"id": "minimax", "source": "minimax-api"},
	  {"id": "commandcode", "source": "commandcode-api", "key": "command"},
	  {"id": "ollama", "source": "ollama-api"},
	  {"id": "orca", "source": "orcarouter-api"},
	  {"id": "workers-ai", "source": "cloudflare-api", "account": "0123456789abcdef0123456789abcdef"},
	  {"id": "moonshot", "source": "moonshot-api"},
	  {"id": "kimi", "source": "kimi-code-api"},
	  {"id": "xai", "source": "xai-api", "team": "team-e2e"},
	  {"id": "cursor", "source": "cursor-admin-api", "email": "member@example.com"}
	]}`, nil)
	origin, received := fakeProviders(t)
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", origin)
	const phrase = "a synthetic passphrase"
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatalf("init = %d %s", code, stderr)
	}
	for _, name := range []string{"openrouter", "vercel", "zai", "minimax", "command", "ollama", "orca", "workers-ai", "moonshot", "kimi", "xai", "cursor"} {
		if code, _, stderr := tr.runWithPassphrase(bin, "sk-synthetic-"+name+"\n", phrase, "keys", "set", name); code != 0 {
			t.Fatalf("keys set %s = %d %s", name, code, stderr)
		}
	}
	if code, _, stderr := tr.runWithPassphrase(bin, "", phrase, "collect"); code != 0 {
		t.Fatalf("collect = %d %s", code, stderr)
	}
	// The model catalogue is public: it never receives a key.
	if got := received()["/api.json"]; got != " | ataecina/"+versionOf(t) {
		t.Fatalf("the model catalogue received %q", got)
	}
	for path, header := range received() {
		if path == "/api.json" {
			continue
		}
		if path == "/teams/spend" {
			// Cursor takes the key as the Basic user name.
			if !strings.HasPrefix(header, "Basic "+base64.StdEncoding.EncodeToString([]byte("sk-synthetic-cursor:"))) {
				t.Fatalf("Cursor received %q", header)
			}
			continue
		}
		if !strings.HasPrefix(header, "Bearer sk-synthetic-") || !strings.HasSuffix(header, "| ataecina/"+versionOf(t)) {
			t.Fatalf("%s received %q", path, header)
		}
	}
	if got := received()["/v1/credits"]; !strings.HasPrefix(got, "Bearer sk-synthetic-vercel ") {
		t.Fatalf("Vercel received another provider's key: %q", got)
	}

	_, out, _ := tr.run(bin, "status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []struct {
				ID      string  `json:"id"`
				Outcome string  `json:"outcome"`
				Plan    *string `json:"plan"`
				Meters  []struct {
					Window      string  `json:"window"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"meters"`
				Balance *struct {
					Remaining *struct {
						Amount float64 `json:"amount"`
					} `json:"remaining"`
				} `json:"balance"`
				Offerings *struct {
					Catalogue string `json:"catalogue"`
					Models    int    `json:"models"`
					Free      int    `json:"free"`
				} `json:"offerings"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	byID := map[string]int{}
	for i, s := range doc.Data.Subscriptions {
		byID[s.ID] = i
		if s.Outcome != "OK" {
			t.Fatalf("%s = %s", s.ID, s.Outcome)
		}
	}
	subscriptions := doc.Data.Subscriptions
	if b := subscriptions[byID["openrouter"]].Balance; b == nil || b.Remaining.Amount != 8.42 {
		t.Fatalf("openrouter balance = %+v", b)
	}
	if o := subscriptions[byID["openrouter"]].Offerings; o == nil || o.Catalogue != "openrouter" || o.Models != 3 || o.Free != 1 {
		t.Fatalf("openrouter offerings = %+v", o)
	}
	if o := subscriptions[byID["zai"]].Offerings; o == nil || o.Catalogue != "zai-coding-plan" || o.Free != o.Models {
		t.Fatalf("zai offerings = %+v", o)
	}
	if b := subscriptions[byID["moonshot"]].Balance; b == nil || b.Remaining.Amount != 12.75 {
		t.Fatalf("moonshot balance = %+v", b)
	}
	if kimi := subscriptions[byID["kimi"]]; *kimi.Plan != "INTERMEDIATE" || len(kimi.Meters) != 2 {
		t.Fatalf("kimi = %+v", kimi)
	}
	if b := subscriptions[byID["xai"]].Balance; b == nil || b.Remaining.Amount != 17.5 {
		t.Fatalf("xai balance = %+v", b)
	}
	if cursor := subscriptions[byID["cursor"]]; len(cursor.Meters) != 1 || cursor.Balance == nil || cursor.Balance.Remaining.Amount != 37.495 {
		t.Fatalf("cursor = %+v", cursor)
	}
	if b := subscriptions[byID["vercel"]].Balance; b == nil || b.Remaining.Amount != 95.5 {
		t.Fatalf("vercel balance = %+v", b)
	}
	if zai := subscriptions[byID["zai"]]; *zai.Plan != "LITE" || len(zai.Meters) != 2 || zai.Meters[1].UsedPercent != 41 {
		t.Fatalf("zai = %+v", zai)
	}
	if command := subscriptions[byID["commandcode"]]; *command.Plan != "INDIVIDUAL_PRO" || len(command.Meters) != 3 {
		t.Fatalf("commandcode = %+v", command)
	}
	if ollama := subscriptions[byID["ollama"]]; len(ollama.Meters) != 1 || ollama.Meters[0].UsedPercent != 25 || ollama.Balance == nil || ollama.Balance.Remaining.Amount != 50.5 {
		t.Fatalf("ollama = %+v", ollama)
	}
	if b := subscriptions[byID["orca"]].Balance; b == nil || b.Remaining.Amount != 8.42 {
		t.Fatalf("orca balance = %+v", b)
	}
	if workers := subscriptions[byID["workers-ai"]]; len(workers.Meters) != 1 || workers.Meters[0].Window != "DAILY" || workers.Meters[0].UsedPercent != 33.505 {
		t.Fatalf("workers-ai = %+v", workers)
	}
	keysJSON, _ := os.ReadFile(filepath.Join(tr.root, "config", "ataecina", "keys.json"))
	snapshot, _ := os.ReadFile(filepath.Join(tr.root, "run", "ataecina", "snapshot.json"))
	logged, _ := os.ReadFile(filepath.Join(tr.root, "state", "ataecina", "ataecina.log"))
	for name, data := range map[string][]byte{"keys.json": keysJSON, "snapshot": snapshot, "log": logged, "status": []byte(out)} {
		if strings.Contains(string(data), "sk-synthetic") {
			t.Fatalf("a key leaked into the %s", name)
		}
	}
}

func TestWithoutUnlockingTheCollectorReportsLocked(t *testing.T) {
	bin := binary(t, "ATAECINA_E2E_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [{"id": "vercel", "source": "vercel-api"}]}`, nil)
	origin, _ := fakeProviders(t)
	tr.setenv("ATAECINA_E2E_PROVIDER_ORIGIN", origin)
	if code, _, _ := tr.runWithPassphrase(bin, "", "a synthetic passphrase", "init", "--unlocker", "passphrase"); code != 0 {
		t.Fatal("init")
	}
	if code, _, _ := tr.runWithPassphrase(bin, "sk-synthetic-vercel\n", "a synthetic passphrase", "keys", "set", "vercel"); code != 0 {
		t.Fatal("keys set")
	}
	// No terminal, no --passphrase-fd, no keyring: the master key stays closed.
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d %s", code, stderr)
	}
	_, out, _ := tr.run(bin, "status", "--json")
	if !strings.Contains(out, `"outcome": "LOCKED"`) {
		t.Fatalf("status:\n%s", out)
	}
}

func versionOf(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}
