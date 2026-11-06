// Package alibabacli reads the Alibaba Model Studio Token Plan with the
// official Bailian CLI (SPECS §3): `bl usage token-plan --output json`,
// with bl's own login. The answer is one flat object whose windows are
// optional and depend on the bl version: per5Hour, per1Week and per1Month,
// each a used fraction (0–1, "…Percentage") and a reset instant in epoch
// milliseconds ("…ResetTime"); hasQuotaData false means the account has no
// Token Plan quota to show.
package alibabacli

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// SourceID is the source this package implements.
const SourceID domain.SourceID = "alibaba-cli"

// Arguments is the fixed argv.
var Arguments = []string{"usage", "token-plan", "--output", "json"}

// ErrNoQuota is bl saying there is no quota data for the account.
var ErrNoQuota = errors.New("bl reports no Token Plan quota data")

// Window is one quota window of the answer.
type Window struct {
	Window   domain.Window
	Fraction float64
	// ResetsAt is zero when bl gives no reset instant.
	ResetsAt time.Time
}

// Source reads one Token Plan subscription.
type Source struct {
	command officialcli.Command
	now     func() time.Time
}

// New runs the bl found with findBl.
func New(findBl func() (string, error), now func() time.Time) Source {
	return Source{now: now, command: officialcli.Command{
		Find: findBl, Args: Arguments, Env: []string{"NO_COLOR=1"},
		Timeout: 15 * time.Second, MaxOutput: 64 << 10,
	}}
}

// Read runs bl once.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	output, err := s.command.Output(ctx)
	switch {
	case errors.Is(err, officialcli.ErrNotFound):
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "bl is not on PATH", err)
	case errors.Is(err, officialcli.ErrTooLarge):
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "bl usage token-plan answered more than 64 KiB", nil)
	case err != nil:
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "bl usage token-plan failed: check that bl is logged in", err)
	}
	windows, err := DecodeUsage(output)
	if errors.Is(err, ErrNoQuota) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, ErrNoQuota.Error(), nil)
	}
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected bl usage token-plan output", err)
	}
	meters := make([]domain.Meter, 0, len(windows))
	for _, window := range windows {
		used, err := domain.UsedPercentFromFraction(window.Fraction)
		if err != nil {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected bl usage token-plan output", err)
		}
		meters = append(meters, domain.NewMeter("", window.Window, used, window.ResetsAt))
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: s.now(), Meters: meters,
	})
}

// DecodeUsage reads the windows bl reports; at least one is required
// unless hasQuotaData is false.
func DecodeUsage(body []byte) ([]Window, error) {
	var document struct {
		HasQuotaData       *bool    `json:"hasQuotaData"`
		Per5HourPercentage *float64 `json:"per5HourPercentage"`
		Per5HourResetTime  *int64   `json:"per5HourResetTime"`
		Per1WeekPercentage *float64 `json:"per1WeekPercentage"`
		Per1WeekResetTime  *int64   `json:"per1WeekResetTime"`
		Per1MonthPercent   *float64 `json:"per1MonthPercentage"`
		Per1MonthResetTime *int64   `json:"per1MonthResetTime"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.HasQuotaData != nil && !*document.HasQuotaData {
		return nil, ErrNoQuota
	}
	var windows []Window
	for _, candidate := range []struct {
		window   domain.Window
		fraction *float64
		resetsAt *int64
	}{
		{domain.FiveHourWindow(), document.Per5HourPercentage, document.Per5HourResetTime},
		{domain.WeeklyWindow(), document.Per1WeekPercentage, document.Per1WeekResetTime},
		{domain.MonthlyWindow(), document.Per1MonthPercent, document.Per1MonthResetTime},
	} {
		if candidate.fraction == nil {
			continue
		}
		if *candidate.fraction < 0 {
			return nil, errors.New("negative percentage")
		}
		window := Window{Window: candidate.window, Fraction: *candidate.fraction}
		if candidate.resetsAt != nil && *candidate.resetsAt > 0 {
			if *candidate.resetsAt < 1e12 || *candidate.resetsAt > 1e14 {
				return nil, errors.New("reset time is not in epoch milliseconds")
			}
			window.ResetsAt = time.UnixMilli(*candidate.resetsAt).UTC()
		}
		windows = append(windows, window)
	}
	if len(windows) == 0 {
		return nil, errors.New("no per5Hour, per1Week or per1Month percentage")
	}
	return windows, nil
}
