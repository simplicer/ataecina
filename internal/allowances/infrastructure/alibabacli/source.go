// Package alibabacli reads the Alibaba Model Studio Token Plan and Coding
// Plan with the official Bailian CLI (SPECS §3.3), with bl's own console
// login: `bl usage token-plan --output json` and `bl usage coding-plan
// --output json`. Model Studio has no quota API for the plan's API key;
// bl is the only way that needs no session of Ataecina's own.
package alibabacli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// SourceID reads the Token Plan; CodingPlanSourceID reads the Coding Plan.
const (
	SourceID           domain.SourceID = "alibaba-cli"
	CodingPlanSourceID domain.SourceID = "alibaba-coding-cli"
)

// ErrNoQuota is bl saying there is no quota data, or no valid
// subscription, for the account.
var ErrNoQuota = errors.New("bl reports no quota for this plan")

// Window is one quota window of the answer.
type Window struct {
	Window   domain.Window
	Fraction float64
	// ResetsAt is zero when bl gives no reset instant.
	ResetsAt time.Time
}

// Usage is the plan bl names, if any, and its windows.
type Usage struct {
	Plan    string
	Windows []Window
}

// plan is one of the two plans bl reads.
type plan struct {
	args   []string
	decode func([]byte) (Usage, error)
	// none is the hint when bl finds no such plan on the account.
	none string
}

func (p plan) command() string { return "bl " + strings.Join(p.args[:2], " ") }

var (
	tokenPlan = plan{
		args: []string{"usage", "token-plan", "--output", "json"}, decode: DecodeUsage,
		none: "bl finds no Token Plan quota on this account; a Coding Plan is read by the alibaba-coding-cli source",
	}
	codingPlan = plan{
		args: []string{"usage", "coding-plan", "--output", "json"}, decode: DecodeCodingPlan,
		none: "bl finds no active Coding Plan on this account; a Token Plan is read by the alibaba-cli source",
	}
)

// Arguments is the fixed argv of the Token Plan.
var Arguments = tokenPlan.args

// Source reads one Model Studio plan.
type Source struct {
	plan    plan
	command officialcli.Command
	now     func() time.Time
}

// New reads the Token Plan with the bl found with findBl.
func New(findBl func() (string, error), now func() time.Time) Source {
	return newSource(tokenPlan, findBl, now)
}

// NewCodingPlan reads the Coding Plan with the bl found with findBl.
func NewCodingPlan(findBl func() (string, error), now func() time.Time) Source {
	return newSource(codingPlan, findBl, now)
}

func newSource(p plan, findBl func() (string, error), now func() time.Time) Source {
	return Source{plan: p, now: now, command: officialcli.Command{
		Find: findBl, Args: p.args, Env: []string{"NO_COLOR=1"},
		Timeout: 15 * time.Second, MaxOutput: 64 << 10,
	}}
}

// Read runs bl once.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	output, err := s.command.Output(ctx)
	name := s.plan.command()
	switch {
	case errors.Is(err, officialcli.ErrNotFound):
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "bl is not on PATH", err)
	case errors.Is(err, officialcli.ErrTooLarge):
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, name+" answered more than 64 KiB", nil)
	case err != nil:
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, name+" failed: check that bl is logged in (bl auth login --console)", err)
	}
	usage, err := s.plan.decode(output)
	if errors.Is(err, ErrNoQuota) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, s.plan.none, nil)
	}
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected "+name+" output", err)
	}
	meters := make([]domain.Meter, 0, len(usage.Windows))
	for _, window := range usage.Windows {
		used, err := domain.UsedPercentFromFraction(window.Fraction)
		if err != nil {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected "+name+" output", err)
		}
		meters = append(meters, domain.NewMeter("", window.Window, used, window.ResetsAt))
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: domain.NewPlan(usage.Plan), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: s.now(), Meters: meters,
	})
}

var tokenPlanField = regexp.MustCompile(`^per(\d{1,3})(Hour|Day|Week|Month)(Percentage|ResetTime)$`)

// DecodeUsage reads the Token Plan's flat answer: every
// per<N><Hour|Day|Week|Month>Percentage (a used fraction, 0–1) with its
// …ResetTime (epoch ms), whatever windows the bl version reports;
// hasQuotaData false means there is no quota to show.
func DecodeUsage(body []byte) (Usage, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return Usage{}, err
	}
	if raw, ok := document["hasQuotaData"]; ok && string(raw) == "false" {
		return Usage{}, ErrNoQuota
	}
	type slot struct {
		fraction *float64
		resetsAt *int64
	}
	slots := map[string]*slot{}
	for name, raw := range document {
		match := tokenPlanField.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		key := match[1] + match[2]
		if slots[key] == nil {
			slots[key] = &slot{}
		}
		if match[3] == "Percentage" {
			var fraction float64
			if err := json.Unmarshal(raw, &fraction); err != nil {
				return Usage{}, err
			}
			slots[key].fraction = &fraction
			continue
		}
		var resetsAt int64
		if err := json.Unmarshal(raw, &resetsAt); err != nil {
			return Usage{}, err
		}
		slots[key].resetsAt = &resetsAt
	}
	var usage Usage
	for key, found := range slots {
		if found.fraction == nil {
			continue
		}
		window, ok := tokenPlanWindow(key)
		if !ok {
			continue
		}
		read, err := newWindow(window, *found.fraction, found.resetsAt)
		if err != nil {
			return Usage{}, err
		}
		usage.Windows = append(usage.Windows, read)
	}
	if len(usage.Windows) == 0 {
		return Usage{}, errors.New("no per<N><unit>Percentage window")
	}
	sortWindows(usage.Windows)
	return usage, nil
}

// tokenPlanWindow maps "5Hour", "1Week", "1Month"… to a window; a span of
// several months has no fixed length and is skipped.
func tokenPlanWindow(key string) (domain.Window, bool) {
	for unit, minutes := range map[string]int{"Hour": 60, "Day": 1440, "Week": 10080} {
		if count, found := strings.CutSuffix(key, unit); found {
			n, _ := strconv.Atoi(count)
			window, err := domain.WindowOfMinutes(n * minutes)
			return window, err == nil
		}
	}
	return domain.MonthlyWindow(), key == "1Month"
}

// DecodeCodingPlan reads the Coding Plan's answer: per5Hour, perWeek and
// perBillMonth, each with usedQuota, totalQuota, percentage (a used
// fraction) and resetTime (epoch ms), and instanceType, the plan. An
// empty answer means there is no valid subscription.
func DecodeCodingPlan(body []byte) (Usage, error) {
	type quota struct {
		UsedQuota  *float64 `json:"usedQuota"`
		TotalQuota *float64 `json:"totalQuota"`
		Percentage *float64 `json:"percentage"`
		ResetTime  *int64   `json:"resetTime"`
	}
	var document struct {
		InstanceType string `json:"instanceType"`
		Per5Hour     *quota `json:"per5Hour"`
		PerWeek      *quota `json:"perWeek"`
		PerBillMonth *quota `json:"perBillMonth"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Usage{}, err
	}
	usage := Usage{Plan: document.InstanceType}
	for _, candidate := range []struct {
		window domain.Window
		quota  *quota
	}{
		{domain.FiveHourWindow(), document.Per5Hour},
		{domain.WeeklyWindow(), document.PerWeek},
		{domain.BillingCycleWindow(), document.PerBillMonth},
	} {
		if candidate.quota == nil {
			continue
		}
		fraction, err := codingFraction(candidate.quota.Percentage, candidate.quota.UsedQuota, candidate.quota.TotalQuota)
		if err != nil {
			return Usage{}, err
		}
		window, err := newWindow(candidate.window, fraction, candidate.quota.ResetTime)
		if err != nil {
			return Usage{}, err
		}
		usage.Windows = append(usage.Windows, window)
	}
	if len(usage.Windows) == 0 {
		if document.InstanceType == "" {
			return Usage{}, ErrNoQuota
		}
		return Usage{}, errors.New("no per5Hour, perWeek or perBillMonth quota")
	}
	return usage, nil
}

// codingFraction is the percentage bl gives, else used ÷ total.
func codingFraction(percentage, used, total *float64) (float64, error) {
	if percentage != nil {
		return *percentage, nil
	}
	if used == nil || total == nil || *total <= 0 {
		return 0, errors.New("a quota needs percentage, or usedQuota and totalQuota")
	}
	return *used / *total, nil
}

func newWindow(window domain.Window, fraction float64, resetsAt *int64) (Window, error) {
	if fraction < 0 {
		return Window{}, errors.New("negative percentage")
	}
	read := Window{Window: window, Fraction: fraction}
	if resetsAt != nil && *resetsAt > 0 {
		if *resetsAt < 1e12 || *resetsAt > 1e14 {
			return Window{}, errors.New("reset time is not in epoch milliseconds")
		}
		read.ResetsAt = time.UnixMilli(*resetsAt).UTC()
	}
	return read, nil
}

// sortWindows puts the shortest first; calendar windows come last.
func sortWindows(windows []Window) {
	length := func(w Window) int {
		if w.Window.Minutes() == 0 {
			return 1 << 30
		}
		return w.Window.Minutes()
	}
	slices.SortFunc(windows, func(a, b Window) int { return cmp.Compare(length(a), length(b)) })
}
