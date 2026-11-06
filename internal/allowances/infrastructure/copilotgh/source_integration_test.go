//go:build integration

package copilotgh_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/copilotgh"
)

var (
	now          = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)
	subscription = domain.NewSubscription("copilot", "github", "copilot-gh", domain.FidelityDerived, "")
)

// fakeGh points the adapter at tests/e2e/testdata/fake-gh with a report.
func fakeGh(t *testing.T, report string, fail bool) (copilotgh.Source, string) {
	t.Helper()
	dir := t.TempDir()
	journal, reportPath := filepath.Join(dir, "journal"), filepath.Join(dir, "report.json")
	if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_GH_JOURNAL", journal)
	t.Setenv("FAKE_GH_REPORT", reportPath)
	failure := ""
	if fail {
		failure = "1"
	}
	t.Setenv("FAKE_GH_FAIL", failure)
	gh, err := filepath.Abs("../../../../tests/e2e/testdata/fake-gh")
	if err != nil {
		t.Fatal(err)
	}
	return copilotgh.New(func() (string, error) { return gh, nil }, "octo-cat", 300, func() time.Time { return now }), journal
}

func TestTheMonthIsReadThroughGh(t *testing.T) {
	source, journal := fakeGh(t, string(read(t, "report.json")), false)
	reading, err := source.Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meter := reading.Meters()[0]
	resetsAt, _ := meter.ResetsAt()
	if meter.Window() != domain.MonthlyWindow() || meter.Used() != 55 || !resetsAt.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("meter = %+v, %v", meter, resetsAt)
	}
	balance, _ := reading.Balance()
	spent, _ := balance.Spent()
	if spent.Amount().String() != "0.6" || reading.Fidelity() != domain.FidelityDerived {
		t.Fatalf("spent = %v, fidelity %s", spent, reading.Fidelity())
	}
	called, _ := os.ReadFile(journal)
	if strings.TrimSpace(string(called)) != "api --method GET -H X-GitHub-Api-Version: 2026-03-10 /users/octo-cat/settings/billing/premium_request/usage?year=2026&month=11" {
		t.Fatalf("gh was called with %q", called)
	}
}

func TestAQuietMonthSaysSo(t *testing.T) {
	source, _ := fakeGh(t, `{"usageItems": []}`, false)
	reading, err := source.Read(context.Background(), subscription)
	if err != nil || reading.Meters()[0].Used() != 0 || reading.Hint() == "" {
		t.Fatalf("Read = %+v, %v", reading, err)
	}
}

// expectClass reads once and checks the failure class.
func expectClass(t *testing.T, name string, source copilotgh.Source, class domain.ErrorClass) {
	t.Helper()
	_, err := source.Read(context.Background(), subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != class {
		t.Fatalf("%s: %v, want %s", name, err, class)
	}
}

// Each fake is made right before it is read: its answer lives in the
// process environment.
func TestGhFailuresAreClassified(t *testing.T) {
	missing := copilotgh.New(func() (string, error) { return "", errors.New("not found") }, "octo-cat", 300, time.Now)
	expectClass(t, "missing gh", missing, domain.ErrorUnreadable)
	failing, _ := fakeGh(t, "{}", true)
	expectClass(t, "gh fails", failing, domain.ErrorUnavailable)
	changed, _ := fakeGh(t, `{"report": "moved"}`, false)
	expectClass(t, "a changed report", changed, domain.ErrorContractChanged)
}
