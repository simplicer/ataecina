// Package copilotgh reads GitHub Copilot usage with the official `gh` CLI
// (SPECS §3.3): `gh api` asks GitHub's documented premium request usage
// report of the owner's login for this month, with gh's own login, so
// Ataecina never sees a token. Used percentage = the Copilot gross
// quantity ÷ the monthly allowance the owner declared (DERIVED); the
// MONTHLY window is reborn on the 1st at 00:00 UTC; the net amount is the
// overage spent.
package copilotgh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// SourceID is the source this package implements.
const SourceID domain.SourceID = "copilot-gh"

// apiVersion is the GitHub REST API version the report is read with.
const apiVersion = "2026-03-10"

// Usage is the Copilot part of one month's report.
type Usage struct {
	Quantity  float64
	NetAmount float64
	// Items counts the Copilot rows; 0 means nothing was reported.
	Items int
}

// Source reads one Copilot subscription.
type Source struct {
	// gh is the invocation without its argv, which depends on the month.
	gh        officialcli.Command
	login     string
	allowance float64
	now       func() time.Time
}

// New reads the report of login against a declared monthly allowance.
func New(findGh func() (string, error), login string, allowance float64, now func() time.Time) Source {
	gh := officialcli.Command{
		Find:    findGh,
		Env:     []string{"GH_NO_UPDATE_NOTIFIER=1", "GH_PROMPT_DISABLED=1", "NO_COLOR=1"},
		Timeout: 15 * time.Second, MaxOutput: 1 << 20,
	}
	return Source{gh: gh, login: login, allowance: allowance, now: now}
}

// unexpected is the failure for a report whose shape is not the known one.
func unexpected(err error) error {
	return domain.Failure(domain.ErrorContractChanged, "unexpected billing report from gh api", err)
}

// Arguments is the fixed argv for one month; login is validated by the
// configuration, so it is a plain path segment.
func Arguments(login string, month time.Time) []string {
	path := fmt.Sprintf("/users/%s/settings/billing/premium_request/usage?year=%d&month=%d", login, month.Year(), int(month.Month()))
	return []string{"api", "--method", "GET", "-H", "X-GitHub-Api-Version: " + apiVersion, path}
}

// Read runs `gh api` once and derives the month's usage.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	now := s.now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	gh := s.gh
	gh.Args = Arguments(s.login, month)
	output, err := gh.Output(ctx)
	switch {
	case errors.Is(err, officialcli.ErrNotFound):
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "gh is not on PATH", err)
	case errors.Is(err, officialcli.ErrTooLarge):
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "gh api answered more than 1 MiB", nil)
	case err != nil:
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable,
			"gh api failed: check `gh auth status` (the login needs to read its billing) and the \"login\" in config.json", err)
	}
	usage, err := DecodeReport(output)
	if err != nil {
		return domain.Reading{}, unexpected(err)
	}
	used, err := domain.NewUsedPercent(math.Min(usage.Quantity*100/s.allowance, 10000))
	if err != nil {
		return domain.Reading{}, unexpected(err)
	}
	spent, err := providerhttp.Dollars(usage.NetAmount)
	if err != nil {
		return domain.Reading{}, unexpected(err)
	}
	balance, err := domain.NewBalance(nil, &spent)
	if err != nil {
		return domain.Reading{}, err
	}
	hint := ""
	if usage.Items == 0 {
		hint = "no Copilot usage in this month's premium request report"
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityDerived, Outcome: domain.OutcomeOK, ObservedAt: now,
		Meters:  []domain.Meter{domain.NewMeter("", domain.MonthlyWindow(), used, month.AddDate(0, 1, 0))},
		Balance: &balance, Hint: hint,
	})
}

// DecodeReport sums grossQuantity and netAmount of the Copilot items.
func DecodeReport(body []byte) (Usage, error) {
	var document struct {
		UsageItems *[]struct {
			Product       string   `json:"product"`
			GrossQuantity *float64 `json:"grossQuantity"`
			NetAmount     *float64 `json:"netAmount"`
		} `json:"usageItems"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Usage{}, err
	}
	if document.UsageItems == nil {
		return Usage{}, errors.New("usageItems is required")
	}
	var usage Usage
	for _, item := range *document.UsageItems {
		if !strings.EqualFold(item.Product, "copilot") {
			continue
		}
		if item.GrossQuantity == nil || item.NetAmount == nil || *item.GrossQuantity < 0 || *item.NetAmount < 0 {
			return Usage{}, errors.New("grossQuantity and netAmount are required and not negative")
		}
		usage.Quantity += *item.GrossQuantity
		usage.NetAmount += *item.NetAmount
		usage.Items++
	}
	return usage, nil
}
