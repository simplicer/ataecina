//go:build integration

package opencodeconsole_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/opencodeconsole"
)

var (
	now          = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	subscription = domain.NewSubscription("opencode-go", "opencode", "opencode-console", domain.FidelityReported, "")
)

// fakeOpencode points the adapter at tests/e2e/testdata/fake-opencode with
// an answer.
func fakeOpencode(t *testing.T, answer string, fail bool) (opencodeconsole.Source, string) {
	t.Helper()
	dir := t.TempDir()
	journal, limits := filepath.Join(dir, "journal"), filepath.Join(dir, "limits.json")
	if err := os.WriteFile(limits, []byte(answer), 0o600); err != nil {
		t.Fatal(err)
	}
	failure := ""
	if fail {
		failure = "1"
	}
	t.Setenv("FAKE_OPENCODE_JOURNAL", journal)
	t.Setenv("FAKE_OPENCODE_LIMITS", limits)
	t.Setenv("FAKE_OPENCODE_FAIL", failure)
	opencode, err := filepath.Abs("../../../../tests/e2e/testdata/fake-opencode")
	if err != nil {
		t.Fatal(err)
	}
	return opencodeconsole.New(func() (string, error) { return opencode, nil }, func() time.Time { return now }), journal
}

func TestGoAndTheBudgetAreReadThroughOpencode(t *testing.T) {
	source, journal := fakeOpencode(t, string(read(t, "limits.json")), false)
	reading, err := source.Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if len(meters) != 4 || meters[0].Used() != 25 || meters[1].Used() != 40.5 || meters[2].Used() != 12 {
		t.Fatalf("meters = %+v", meters)
	}
	if meters[3].Bucket() != "budget" || meters[3].Used() != 25 || meters[3].Window() != domain.BillingCycleWindow() {
		t.Fatalf("budget meter = %+v", meters[3])
	}
	balance, ok := reading.Balance()
	if !ok {
		t.Fatal("no balance")
	}
	if left, _ := balance.Remaining(); left.Amount().Float64() != 15 {
		t.Fatalf("left = %v", left)
	}
	if reading.Plan() != "GO" {
		t.Fatalf("plan = %q", reading.Plan())
	}
	called, _ := os.ReadFile(journal)
	if strings.TrimSpace(string(called)) != "api experimental.usage.limits --standalone" {
		t.Fatalf("opencode was called with %q", called)
	}
}

func TestALimitedGoWindowIsFull(t *testing.T) {
	window := `{"percent":30,"limited":false,"resetsAt":1794141000000}`
	answer := `{"data":{"budgets":[],"go":{"rolling":{"percent":97,"limited":true,"resetsAt":1794141000000},"weekly":` + window + `,"monthly":` + window + `}}}`
	source, _ := fakeOpencode(t, answer, false)
	reading, err := source.Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	if meters := reading.Meters(); meters[0].Used() != 100 {
		t.Fatalf("limited window = %+v", meters[0])
	}
	if _, ok := reading.Balance(); ok {
		t.Fatal("a balance without a budget")
	}
}

// expectClass reads once and checks the failure class.
func expectClass(t *testing.T, name string, source opencodeconsole.Source, class domain.ErrorClass) {
	t.Helper()
	_, err := source.Read(context.Background(), subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != class {
		t.Fatalf("%s: %v, want %s", name, err, class)
	}
}

// Each fake is made right before it is read: its answer lives in the
// process environment.
func TestOpencodeFailuresAreClassified(t *testing.T) {
	missing := opencodeconsole.New(func() (string, error) { return "", errors.New("not found") }, time.Now)
	expectClass(t, "missing opencode", missing, domain.ErrorUnreadable)
	failing, _ := fakeOpencode(t, "{}", true)
	expectClass(t, "opencode fails", failing, domain.ErrorUnavailable)
	signedOut, _ := fakeOpencode(t, string(read(t, "signed-out.json")), false)
	expectClass(t, "signed out", signedOut, domain.ErrorUnavailable)
	empty, _ := fakeOpencode(t, `{"data":{"workspace":{"id":"w","usageURL":"u"},"budgets":[]}}`, false)
	expectClass(t, "no go, no budget", empty, domain.ErrorUnavailable)
	changed, _ := fakeOpencode(t, `{"limits":"moved"}`, false)
	expectClass(t, "a changed answer", changed, domain.ErrorContractChanged)
}
