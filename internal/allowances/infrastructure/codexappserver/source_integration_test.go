//go:build integration

package codexappserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/codexappserver"
)

// The test binary doubles as a fake `codex app-server`: when
// ATAECINA_FAKE_CODEX names a scenario it speaks the protocol on stdio and
// records every method it receives. The real codex is never run.
func TestMain(m *testing.M) {
	if scenario := os.Getenv("ATAECINA_FAKE_CODEX"); scenario != "" {
		os.Exit(fakeCodex(scenario))
	}
	os.Exit(m.Run())
}

func fakeCodex(scenario string) int {
	if len(os.Args) != 2 || os.Args[1] != "app-server" {
		return 2
	}
	result, err := os.ReadFile(os.Getenv("ATAECINA_FAKE_CODEX_RESULT"))
	if err != nil {
		return 3
	}
	journal, err := os.OpenFile(os.Getenv("ATAECINA_FAKE_CODEX_JOURNAL"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 4
	}
	defer journal.Close()
	out := bufio.NewWriter(os.Stdout)
	reply := func(line string) {
		fmt.Fprintln(out, line)
		out.Flush()
	}
	lines := bufio.NewScanner(os.Stdin)
	for lines.Scan() {
		var request struct {
			Method string `json:"method"`
			ID     *int   `json:"id"`
		}
		if json.Unmarshal(lines.Bytes(), &request) != nil {
			return 5
		}
		fmt.Fprintln(journal, request.Method)
		switch request.Method {
		case "initialize":
			reply(`{"method":"account/updated","params":{}}`)
			reply(fmt.Sprintf(`{"id":%d,"result":{"userAgent":"codex/fake"}}`, *request.ID))
		case "account/rateLimits/read":
			switch scenario {
			case "ok":
				reply(`{"id":99,"method":"item/tool/requestUserInput","params":{}}`)
				reply(`{"method":"note","params":{"text":"` + strings.Repeat("x", 1100<<10) + `"}}`)
				reply(fmt.Sprintf(`{"id":%d,"result":%s}`, *request.ID, strings.TrimSpace(string(result))))
			case "unknown-method":
				reply(fmt.Sprintf(`{"id":%d,"error":{"code":-32601,"message":"unknown"}}`, *request.ID))
			case "not-logged-in":
				reply(fmt.Sprintf(`{"id":%d,"error":{"code":-32000,"message":"login required"}}`, *request.ID))
			case "garbage":
				reply(fmt.Sprintf(`{"id":%d,"result":{"rateLimits":{"primary":{"usedPercent":"x"}}}}`, *request.ID))
			case "no-figures":
				reply(fmt.Sprintf(`{"id":%d,"result":{"rateLimits":{"limitId":"codex"}}}`, *request.ID))
			case "silent":
				return 0
			case "hang":
				time.Sleep(time.Minute)
			}
		}
	}
	return 0
}

type fake struct {
	t       *testing.T
	journal string
}

func newFake(t *testing.T, scenario string) *fake {
	t.Helper()
	result, err := filepath.Abs("testdata/rate-limits-read.json")
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "methods")
	t.Setenv("ATAECINA_FAKE_CODEX", scenario)
	t.Setenv("ATAECINA_FAKE_CODEX_RESULT", result)
	t.Setenv("ATAECINA_FAKE_CODEX_JOURNAL", journal)
	return &fake{t: t, journal: journal}
}

func (f *fake) read(ctx context.Context) (domain.Reading, error) {
	source := codexappserver.New(func() (string, error) { return os.Args[0], nil }, "0.3.0-unstable", func() time.Time { return answeredAt })
	return source.Read(ctx, domain.NewSubscription("codex", "openai", codexappserver.SourceID, domain.FidelityReported, ""))
}

func (f *fake) methods() []string {
	data, _ := os.ReadFile(f.journal)
	return strings.Fields(string(data))
}

func TestTheAppServerGivesLimitsAndBankedResets(t *testing.T) {
	f := newFake(t, "ok")
	reading, err := f.read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(reading.Meters()) != 3 || reading.Plan() != "PLUS" || reading.Source() != codexappserver.SourceID {
		t.Fatalf("reading = %+v", reading.Meters())
	}
	if banked, ok := reading.BankedResets(); !ok || banked.Available() != 2 {
		t.Fatal("banked resets are shown as information")
	}
	if credits, ok := reading.Credits(); !ok || credits.Available().String() != "7.5" {
		t.Fatalf("credits = %+v", credits)
	}
	got := strings.Join(f.methods(), " ")
	if got != "initialize initialized account/rateLimits/read" {
		t.Fatalf("methods sent = %q", got)
	}
}

func TestAppServerFailuresAreClassified(t *testing.T) {
	cases := map[string]domain.ErrorClass{
		"unknown-method": domain.ErrorContractChanged,
		"not-logged-in":  domain.ErrorUnavailable,
		"garbage":        domain.ErrorContractChanged,
		"no-figures":     domain.ErrorUnreadable,
		"silent":         domain.ErrorUnavailable,
	}
	for scenario, class := range cases {
		f := newFake(t, scenario)
		_, err := f.read(context.Background())
		var failure *domain.SourceFailure
		if !errors.As(err, &failure) || failure.Class() != class {
			t.Fatalf("%s: %v, want %s", scenario, err, class)
		}
		for _, method := range f.methods() {
			if method != "initialize" && method != "initialized" && method != "account/rateLimits/read" {
				t.Fatalf("%s: forbidden method %q sent", scenario, method)
			}
		}
	}
}

func TestAHangingAppServerIsStoppedAtTheDeadline(t *testing.T) {
	f := newFake(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := f.read(ctx)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnavailable {
		t.Fatalf("hang = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the session took %v", elapsed)
	}
}

func TestMissingCodexIsUnreadable(t *testing.T) {
	source := codexappserver.New(func() (string, error) { return "", errors.New("not found") }, "x", time.Now)
	_, err := source.Read(context.Background(), domain.NewSubscription("codex", "openai", codexappserver.SourceID, domain.FidelityReported, ""))
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != domain.ErrorUnreadable {
		t.Fatalf("missing codex = %v", err)
	}
	broken := codexappserver.New(func() (string, error) { return filepath.Join(t.TempDir(), "codex"), nil }, "x", time.Now)
	if _, err := broken.Read(context.Background(), domain.NewSubscription("codex", "openai", codexappserver.SourceID, domain.FidelityReported, "")); !errors.As(err, &failure) {
		t.Fatalf("unstartable codex = %v", err)
	}
}
