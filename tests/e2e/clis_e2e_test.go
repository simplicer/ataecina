//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeCLI copies tests/e2e/testdata/fake-<name> to <tree>/bin/<name>
// with its answer, and returns the file its argv is journalled to.
func installFakeCLI(t *testing.T, tr *tree, name, variable, answer string) string {
	t.Helper()
	script, err := os.ReadFile("testdata/fake-" + name)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tr.root, "bin")
	mustPrivateDir(t, bin)
	if err := os.WriteFile(filepath.Join(bin, name), script, 0o700); err != nil {
		t.Fatal(err)
	}
	answerFile := filepath.Join(tr.root, name+"-answer.json")
	if err := os.WriteFile(answerFile, []byte(answer), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(tr.root, name+"-argv")
	prefix := "FAKE_" + strings.ToUpper(name)
	tr.setenv("PATH", bin+":/usr/bin:/bin")
	tr.setenv(prefix+"_"+variable, answerFile)
	tr.setenv(prefix+"_JOURNAL", journal)
	return journal
}

// The production binary reads Copilot and the Alibaba Token Plan through
// fake `gh` and `bl`; the real CLIs, GitHub and Alibaba are never reached.
func TestOfficialCLIsAreRunWithAFixedArgv(t *testing.T) {
	bin := binary(t, "ATAECINA_PRODUCTION_BINARY")
	tr := newTree(t, `{"version": 1, "subscriptions": [
	  {"id": "copilot", "source": "copilot-gh", "login": "octo-cat", "monthly_allowance": 300},
	  {"id": "alibaba", "source": "alibaba-cli"}
	]}`, nil)
	ghJournal := installFakeCLI(t, tr, "gh", "REPORT", fixture(t, "copilotgh/testdata/report.json"))
	blJournal := installFakeCLI(t, tr, "bl", "USAGE", fixture(t, "alibabacli/testdata/token-plan.json"))
	if code, _, stderr := tr.run(bin, "collect"); code != 0 {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
	for journal, want := range map[string]string{
		ghJournal: "api --method GET -H X-GitHub-Api-Version: 2026-03-10 /users/octo-cat/settings/billing/premium_request/usage?year=",
		blJournal: "usage token-plan --output json",
	} {
		called, _ := os.ReadFile(journal)
		if !strings.HasPrefix(string(called), want) || strings.Count(string(called), "\n") != 1 {
			t.Fatalf("argv = %q, want %q once", called, want)
		}
	}
	_, out, _ := tr.run(bin, "status", "--json")
	var doc struct {
		Data struct {
			Subscriptions []struct {
				ID       string `json:"id"`
				Outcome  string `json:"outcome"`
				Fidelity string `json:"fidelity"`
				Meters   []struct {
					Window      string  `json:"window"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"meters"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	copilot, alibaba := doc.Data.Subscriptions[0], doc.Data.Subscriptions[1]
	if copilot.Outcome != "OK" || copilot.Fidelity != "DERIVED" || copilot.Meters[0].Window != "MONTHLY" || copilot.Meters[0].UsedPercent != 55 {
		t.Fatalf("copilot = %+v", copilot)
	}
	if alibaba.Outcome != "OK" || len(alibaba.Meters) != 2 || alibaba.Meters[1].UsedPercent != 70 {
		t.Fatalf("alibaba = %+v", alibaba)
	}
}
