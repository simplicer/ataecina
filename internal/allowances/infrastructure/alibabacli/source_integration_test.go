//go:build integration

package alibabacli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/alibabacli"
)

var (
	now          = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)
	subscription = domain.NewSubscription("alibaba", "alibaba", "alibaba-cli", domain.FidelityReported, "")
)

// fakeBl points the adapter at tests/e2e/testdata/fake-bl with an answer.
func fakeBl(t *testing.T, answer string, fail bool) (alibabacli.Source, string) {
	t.Helper()
	dir := t.TempDir()
	journal, usage := filepath.Join(dir, "journal"), filepath.Join(dir, "usage.json")
	if err := os.WriteFile(usage, []byte(answer), 0o600); err != nil {
		t.Fatal(err)
	}
	failure := ""
	if fail {
		failure = "1"
	}
	t.Setenv("FAKE_BL_JOURNAL", journal)
	t.Setenv("FAKE_BL_USAGE", usage)
	t.Setenv("FAKE_BL_FAIL", failure)
	bl, err := filepath.Abs("../../../../tests/e2e/testdata/fake-bl")
	if err != nil {
		t.Fatal(err)
	}
	return alibabacli.New(func() (string, error) { return bl, nil }, func() time.Time { return now }), journal
}

func TestTheTokenPlanIsReadThroughBl(t *testing.T) {
	source, journal := fakeBl(t, string(read(t, "token-plan.json")), false)
	reading, err := source.Read(context.Background(), subscription)
	if err != nil {
		t.Fatal(err)
	}
	meters := reading.Meters()
	if len(meters) != 2 || meters[0].Used() != 12.5 || meters[1].Used() != 70 {
		t.Fatalf("meters = %+v", meters)
	}
	called, _ := os.ReadFile(journal)
	if strings.TrimSpace(string(called)) != "usage token-plan --output json" {
		t.Fatalf("bl was called with %q", called)
	}
}

// expectClass reads once and checks the failure class.
func expectClass(t *testing.T, name string, source alibabacli.Source, class domain.ErrorClass) {
	t.Helper()
	_, err := source.Read(context.Background(), subscription)
	var failure *domain.SourceFailure
	if !errors.As(err, &failure) || failure.Class() != class {
		t.Fatalf("%s: %v, want %s", name, err, class)
	}
}

// Each fake is made right before it is read: its answer lives in the
// process environment.
func TestBlFailuresAreClassified(t *testing.T) {
	missing := alibabacli.New(func() (string, error) { return "", errors.New("not found") }, time.Now)
	expectClass(t, "missing bl", missing, domain.ErrorUnreadable)
	failing, _ := fakeBl(t, "{}", true)
	expectClass(t, "bl fails", failing, domain.ErrorUnavailable)
	empty, _ := fakeBl(t, `{"hasQuotaData": false}`, false)
	expectClass(t, "no quota data", empty, domain.ErrorUnavailable)
	changed, _ := fakeBl(t, `{"quota": "moved"}`, false)
	expectClass(t, "a changed answer", changed, domain.ErrorContractChanged)
}
