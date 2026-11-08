// Package opencodeconsole reads the OpenCode Go allowance and the OpenCode
// Console budgets with the official OpenCode CLI (SPECS §3.4):
// `opencode api experimental.usage.limits --standalone`, with OpenCode's own
// sign-in to the Console. The answer is `{ "data": null }` when OpenCode is
// not signed in to a Console workspace, otherwise the workspace's Go windows
// (rolling five hours, week, month: percent used, limited, reset instant in
// epoch milliseconds) and its budgets (limit and spent in USD, reset instant).
package opencodeconsole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// SourceID is the source this package implements.
const SourceID domain.SourceID = "opencode-console"

// Arguments is the fixed argv: one experimental operation of OpenCode's
// own server, with a private server so no background service is left
// running.
var Arguments = []string{"api", "experimental.usage.limits", "--standalone"}

// operation names the call in failure messages.
const operation = "opencode api experimental.usage.limits"

// ErrSignedOut is OpenCode saying it is not signed in to a Console workspace.
var ErrSignedOut = errors.New("OpenCode is not signed in to an OpenCode Console workspace")

// GoWindow is one window of the OpenCode Go allowance.
type GoWindow struct {
	Window  domain.Window
	Percent float64
	// Limited is OpenCode saying the window is exhausted.
	Limited bool
	// ResetsAt is zero when OpenCode gives no reset instant.
	ResetsAt time.Time
}

// Budget is one spending budget set in the Console, in USD.
type Budget struct {
	Limit    float64
	Spent    float64
	ResetsAt time.Time
}

// Limits is what the Console workspace reports.
type Limits struct {
	Go      []GoWindow
	Budgets []Budget
}

// Source reads one OpenCode Console workspace.
type Source struct {
	command officialcli.Command
	reuse   *officialcli.Reuse
	now     func() time.Time
}

// New runs the opencode found with findOpencode; with reuse set, recent
// figures are given again instead of starting opencode (nil: always run
// it).
func New(findOpencode func() (string, error), now func() time.Time, reuse *officialcli.Reuse) Source {
	return Source{now: now, reuse: reuse, command: officialcli.Command{
		Find: findOpencode, Args: Arguments, Env: []string{"NO_COLOR=1"},
		Timeout: 15 * time.Second, MaxOutput: 64 << 10,
	}}
}

// Read gives the kept limits while they hold, else runs opencode once.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	limits, err := s.limits(ctx)
	if err != nil {
		return domain.Reading{}, err
	}
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: subscription.Plan(), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: s.now(),
	}
	if len(limits.Go) > 0 && facts.Plan.IsUnknown() {
		facts.Plan = domain.NewPlan("GO")
	}
	if facts.Meters, err = meters(limits); err != nil {
		return domain.Reading{}, unexpected(err)
	}
	if len(facts.Meters) == 0 {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "the OpenCode Console workspace has no Go plan and no budget", nil)
	}
	if len(limits.Budgets) == 1 {
		balance, err := budgetBalance(limits.Budgets[0])
		if err != nil {
			return domain.Reading{}, unexpected(err)
		}
		facts.Balance = &balance
	}
	return domain.NewReading(facts)
}

// limits gives the kept limits while they hold, else asks opencode and
// keeps what it extracted (a signed-out answer too: signing in is rare).
func (s Source) limits(ctx context.Context) (Limits, error) {
	var last keptLimits
	if s.reuse.Recall(&last) {
		limits, err := last.limits()
		switch {
		case err == nil:
			return limits, nil
		case errors.Is(err, ErrSignedOut):
			return Limits{}, signedOut()
		}
	}
	output, err := s.command.Output(ctx)
	switch {
	case errors.Is(err, officialcli.ErrNotFound):
		return Limits{}, domain.Failure(domain.ErrorUnreadable, "opencode is not on PATH", err)
	case errors.Is(err, officialcli.ErrTooLarge):
		return Limits{}, domain.Failure(domain.ErrorContractChanged, operation+" answered more than 64 KiB", nil)
	case err != nil:
		return Limits{}, domain.Failure(domain.ErrorUnavailable, operation+" did not answer", err)
	}
	limits, err := DecodeLimits(output)
	if errors.Is(err, ErrSignedOut) {
		s.reuse.Keep(keptLimits{SignedOut: true})
		return Limits{}, signedOut()
	}
	if err != nil {
		return Limits{}, unexpected(err)
	}
	s.reuse.Keep(keep(limits))
	return limits, nil
}

func signedOut() *domain.SourceFailure {
	return domain.Failure(domain.ErrorUnavailable, ErrSignedOut.Error()+": sign it in with opencode auth login", nil)
}

func unexpected(err error) *domain.SourceFailure {
	return domain.Failure(domain.ErrorContractChanged, "unexpected "+operation+" output", err)
}

// meters puts the Go windows in the main bucket and each budget with a
// limit in its own bucket ("budget", or "budget_1", "budget_2"… when there
// are several).
func meters(limits Limits) ([]domain.Meter, error) {
	var meters []domain.Meter
	for _, window := range limits.Go {
		percent := window.Percent
		if window.Limited {
			percent = math.Max(percent, 100)
		}
		used, err := domain.NewUsedPercent(percent)
		if err != nil {
			return nil, err
		}
		meters = append(meters, domain.NewMeter("", window.Window, used, window.ResetsAt))
	}
	for i, budget := range limits.Budgets {
		if budget.Limit == 0 {
			continue
		}
		bucket := domain.NewBucketID("budget")
		if len(limits.Budgets) > 1 {
			bucket = domain.NewBucketID(fmt.Sprintf("budget %d", i+1))
		}
		used, err := domain.NewUsedPercent(budget.Spent / budget.Limit * 100)
		if err != nil {
			return nil, err
		}
		meters = append(meters, domain.NewMeter(bucket, domain.BillingCycleWindow(), used, budget.ResetsAt))
	}
	return meters, nil
}

// budgetBalance is what is left of a budget (never below zero) and what
// was spent of it.
func budgetBalance(budget Budget) (domain.Balance, error) {
	usd, _ := domain.NewCurrency("USD")
	spent, err := domain.AmountFromFloat(budget.Spent)
	if err != nil {
		return domain.Balance{}, err
	}
	left, err := domain.AmountFromFloat(math.Max(budget.Limit-budget.Spent, 0))
	if err != nil {
		return domain.Balance{}, err
	}
	remaining, spentMoney := domain.NewMoney(left, usd), domain.NewMoney(spent, usd)
	return domain.NewBalance(&remaining, &spentMoney)
}

type goMeter struct {
	Percent  *float64 `json:"percent"`
	Limited  *bool    `json:"limited"`
	ResetsAt *float64 `json:"resetsAt"`
}

type budgetDocument struct {
	Limit    *float64 `json:"limit"`
	Spent    *float64 `json:"spent"`
	ResetsAt *float64 `json:"resetsAt"`
}

// DecodeLimits reads the answer: `data` is required, null means signed
// out; `budgets` is required, `go` is optional and, when present, has its
// three windows.
func DecodeLimits(body []byte) (Limits, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return Limits{}, err
	}
	data, ok := top["data"]
	if !ok {
		return Limits{}, errors.New("data is required")
	}
	if string(data) == "null" {
		return Limits{}, ErrSignedOut
	}
	var info struct {
		Go *struct {
			Rolling *goMeter `json:"rolling"`
			Weekly  *goMeter `json:"weekly"`
			Monthly *goMeter `json:"monthly"`
		} `json:"go"`
		Budgets *[]budgetDocument `json:"budgets"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return Limits{}, err
	}
	if info.Budgets == nil {
		return Limits{}, errors.New("data.budgets is required")
	}
	var limits Limits
	if info.Go != nil {
		for _, candidate := range []struct {
			window domain.Window
			meter  *goMeter
		}{
			{domain.FiveHourWindow(), info.Go.Rolling},
			{domain.WeeklyWindow(), info.Go.Weekly},
			{domain.MonthlyWindow(), info.Go.Monthly},
		} {
			window, err := decodeGoWindow(candidate.window, candidate.meter)
			if err != nil {
				return Limits{}, err
			}
			limits.Go = append(limits.Go, window)
		}
	}
	for _, document := range *info.Budgets {
		budget, err := decodeBudget(document)
		if err != nil {
			return Limits{}, err
		}
		limits.Budgets = append(limits.Budgets, budget)
	}
	return limits, nil
}

func decodeGoWindow(window domain.Window, meter *goMeter) (GoWindow, error) {
	if meter == nil || meter.Percent == nil || meter.Limited == nil || meter.ResetsAt == nil {
		return GoWindow{}, errors.New("a Go window needs percent, limited and resetsAt")
	}
	if *meter.Percent < 0 || *meter.Percent > 100 {
		return GoWindow{}, errors.New("a Go percent is outside 0–100")
	}
	resetsAt, err := epochMillis(*meter.ResetsAt)
	if err != nil {
		return GoWindow{}, err
	}
	return GoWindow{Window: window, Percent: *meter.Percent, Limited: *meter.Limited, ResetsAt: resetsAt}, nil
}

func decodeBudget(document budgetDocument) (Budget, error) {
	if document.Limit == nil || document.Spent == nil || document.ResetsAt == nil {
		return Budget{}, errors.New("a budget needs limit, spent and resetsAt")
	}
	if *document.Limit < 0 || *document.Spent < 0 || *document.Limit > 1e9 || *document.Spent > 1e9 {
		return Budget{}, errors.New("a budget amount is out of range")
	}
	resetsAt, err := epochMillis(*document.ResetsAt)
	if err != nil {
		return Budget{}, err
	}
	return Budget{Limit: *document.Limit, Spent: *document.Spent, ResetsAt: resetsAt}, nil
}

// epochMillis reads an instant in epoch milliseconds; 0 is unknown.
func epochMillis(value float64) (time.Time, error) {
	if value == 0 {
		return time.Time{}, nil
	}
	if value < 1e12 || value > 1e14 {
		return time.Time{}, errors.New("reset instant is not in epoch milliseconds")
	}
	return time.UnixMilli(int64(value)).UTC(), nil
}
