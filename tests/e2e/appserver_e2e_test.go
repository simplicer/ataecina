//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installFakeCodex puts tests/e2e/testdata/fake-codex on the tree's PATH
// as `codex`, answering account/rateLimits/read with result.
func installFakeCodex(t *testing.T, tr *tree, result string) string {
	t.Helper()
	script, err := os.ReadFile("testdata/fake-codex")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tr.root, "bin")
	mustPrivateDir(t, bin)
	if err := os.WriteFile(filepath.Join(bin, "codex"), script, 0o700); err != nil {
		t.Fatal(err)
	}
	resultFile := filepath.Join(tr.root, "rate-limits.json")
	if err := os.WriteFile(resultFile, []byte(result), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(tr.root, "codex-methods")
	tr.setenv("PATH", bin+":/usr/bin:/bin")
	tr.setenv("FAKE_CODEX_RESULT", resultFile)
	tr.setenv("FAKE_CODEX_JOURNAL", journal)
	return journal
}

func appServerResult(now time.Time) string {
	return fmt.Sprintf(`{"rateLimitsByLimitId":{"codex":{"limitId":"codex","planType":"plus","primary":{"usedPercent":9,"windowDurationMins":300,"resetsAt":%d},"secondary":{"usedPercent":44,"windowDurationMins":10080,"resetsAt":%d},"credits":{"hasCredits":true,"unlimited":false,"balance":"0"}}},"rateLimitResetCredits":{"availableCount":2}}`,
		now.Add(2*time.Hour).Unix(), now.Add(90*time.Hour).Unix())
}

const appServerConfig = `{"version": 1, "subscriptions": [{"id": "codex", "source": "codex-logs", "app_server": true, "app_server_after_minutes": 5}]}`

type codexStatus struct {
	Data struct {
		Subscriptions []struct {
			Source       string `json:"source"`
			BankedResets *struct {
				Available int `json:"available"`
			} `json:"banked_resets"`
			Meters []struct {
				UsedPercent float64 `json:"used_percent"`
			} `json:"meters"`
		} `json:"subscriptions"`
	} `json:"data"`
}

func collectCodex(t *testing.T, tr *tree, bin string) codexStatus {
	t.Helper()
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	_, out, _ := tr.run(bin, "status", "--json")
	var doc codexStatus
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestStaleCodexLogsAskTheAppServer(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, appServerConfig, nil)
	syntheticCodexLog(t, filepath.Join(tr.root, "home", ".codex"), time.Now().UTC().Add(-30*time.Minute))
	journal := installFakeCodex(t, tr, appServerResult(time.Now().UTC()))

	codex := collectCodex(t, tr, bin).Data.Subscriptions[0]
	if codex.Source != "CODEX_APP_SERVER" || codex.BankedResets == nil || codex.BankedResets.Available != 2 || codex.Meters[0].UsedPercent != 9 {
		t.Fatalf("codex = %+v", codex)
	}
	methods, _ := os.ReadFile(journal)
	if got := strings.Fields(string(methods)); strings.Join(got, " ") != "initialize initialized account/rateLimits/read" {
		t.Fatalf("methods sent to codex = %q", got)
	}
}

func TestTheLogsRemainWhenTheAppServerFails(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, appServerConfig, nil)
	syntheticCodexLog(t, filepath.Join(tr.root, "home", ".codex"), time.Now().UTC().Add(-30*time.Minute))
	installFakeCodex(t, tr, `{"unexpected": true}`)

	codex := collectCodex(t, tr, bin).Data.Subscriptions[0]
	if codex.Source != "CODEX_LOGS" || codex.BankedResets != nil {
		t.Fatalf("codex = %+v", codex)
	}
}
