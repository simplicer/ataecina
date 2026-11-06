// Package commandcodeapi reads Command Code with the owner's API key from
// the alpha endpoints its own CLI uses (undocumented, SPECS §3):
// /alpha/billing/credits (credits and the five-hour and weekly windows),
// /alpha/usage/summary (spend in the billing period) and
// /alpha/billing/subscriptions (plan and period end). To verify live.
package commandcodeapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.commandcode.ai"

const (
	creditsPath       = "/alpha/billing/credits"
	summaryPath       = "/alpha/usage/summary"
	subscriptionsPath = "/alpha/billing/subscriptions"
)

// Credits is what /alpha/billing/credits says.
type Credits struct {
	Remaining float64
	Meters    []domain.Meter
}

// Subscription is what /alpha/billing/subscriptions says.
type Subscription struct {
	Plan      string
	PeriodEnd time.Time
}

// Source reads one Command Code subscription.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	now      func() time.Time
}

// New reads with the named key through endpoint.
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, now: now}
}

// Read asks the three endpoints; only the credits are required.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, creditsPath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	credits, err := DecodeCredits(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Meters: credits.Meters,
	}
	remaining, err := providerhttp.Dollars(credits.Remaining)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	spent, spentKnown := s.spent(ctx, key, now)
	plan, planKnown := s.subscription(ctx, key, now)
	if !spentKnown || !planKnown {
		facts.Outcome, facts.Hint = domain.OutcomePartial, "part of Command Code's billing could not be read"
	}
	if planKnown {
		facts.Plan = domain.NewPlan(plan.Plan)
	}
	var spentMoney *domain.Money
	if spentKnown {
		money, err := providerhttp.Dollars(spent)
		if err != nil {
			return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
		}
		spentMoney = &money
		if pool := spent + credits.Remaining; pool > 0 && planKnown {
			used, err := domain.NewUsedPercent(spent / pool * 100)
			if err != nil {
				return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
			}
			facts.Meters = append(facts.Meters, domain.NewMeter("", domain.BillingCycleWindow(), used, plan.PeriodEnd))
		}
	}
	balance, err := domain.NewBalance(&remaining, spentMoney)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	facts.Balance = &balance
	return domain.NewReading(facts)
}

// spent asks for the period's spend; a failure leaves it unknown.
func (s Source) spent(ctx context.Context, key secret.Key, now time.Time) (float64, bool) {
	body, err := providerhttp.Fetch(ctx, s.endpoint, summaryPath, key, now)
	if err != nil {
		return 0, false
	}
	spent, err := DecodeSummary(body)
	return spent, err == nil
}

// subscription asks for the plan and the period end; a failure leaves
// them unknown.
func (s Source) subscription(ctx context.Context, key secret.Key, now time.Time) (Subscription, bool) {
	body, err := providerhttp.Fetch(ctx, s.endpoint, subscriptionsPath, key, now)
	if err != nil {
		return Subscription{}, false
	}
	subscription, err := DecodeSubscriptions(body)
	return subscription, err == nil
}

type window struct {
	Used    *float64 `json:"used"`
	Cap     *float64 `json:"cap"`
	ResetAt *int64   `json:"resetAt"`
}

// DecodeCredits reads {credits: {monthlyCredits, purchasedCredits,
// freeCredits}, windowLimits: {fiveHour, weekly: {used, cap, resetAt}}}.
// Credits are read as dollars (to verify); resetAt is epoch milliseconds.
func DecodeCredits(body []byte) (Credits, error) {
	var document struct {
		Credits *struct {
			Monthly   *float64 `json:"monthlyCredits"`
			Purchased float64  `json:"purchasedCredits"`
			Free      float64  `json:"freeCredits"`
		} `json:"credits"`
		WindowLimits *struct {
			FiveHour *window `json:"fiveHour"`
			Weekly   *window `json:"weekly"`
		} `json:"windowLimits"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Credits{}, err
	}
	if document.Credits == nil || document.Credits.Monthly == nil {
		return Credits{}, errors.New("credits.monthlyCredits is required")
	}
	credits := Credits{Remaining: *document.Credits.Monthly + document.Credits.Purchased + document.Credits.Free}
	if limits := document.WindowLimits; limits != nil {
		for _, w := range []struct {
			limit  *window
			window domain.Window
		}{{limits.FiveHour, domain.FiveHourWindow()}, {limits.Weekly, domain.WeeklyWindow()}} {
			if w.limit == nil || w.limit.Used == nil || w.limit.Cap == nil || *w.limit.Cap <= 0 {
				continue
			}
			used, err := domain.NewUsedPercent(math.Max(0, *w.limit.Used / *w.limit.Cap * 100))
			if err != nil {
				return Credits{}, err
			}
			var resetsAt time.Time
			if w.limit.ResetAt != nil && *w.limit.ResetAt > 0 && *w.limit.ResetAt < math.MaxInt64/1000 {
				resetsAt = time.UnixMilli(*w.limit.ResetAt)
			}
			credits.Meters = append(credits.Meters, domain.NewMeter("", w.window, used, resetsAt))
		}
	}
	return credits, nil
}

// DecodeSummary reads {totalCost, periodBasis}: the spend of the period.
func DecodeSummary(body []byte) (float64, error) {
	var document struct {
		TotalCost *float64 `json:"totalCost"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, err
	}
	if document.TotalCost == nil || *document.TotalCost < 0 {
		return 0, errors.New("totalCost is required")
	}
	return *document.TotalCost, nil
}

// DecodeSubscriptions reads {success, data: {planId, currentPeriodEnd}}.
func DecodeSubscriptions(body []byte) (Subscription, error) {
	var document struct {
		Success bool `json:"success"`
		Data    *struct {
			PlanID           string    `json:"planId"`
			CurrentPeriodEnd time.Time `json:"currentPeriodEnd"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Subscription{}, err
	}
	if !document.Success || document.Data == nil {
		return Subscription{}, errors.New("no active subscription in the answer")
	}
	return Subscription{Plan: document.Data.PlanID, PeriodEnd: document.Data.CurrentPeriodEnd}, nil
}
