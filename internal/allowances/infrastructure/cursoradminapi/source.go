// Package cursoradminapi reads one member's spending on a Cursor Teams or
// Enterprise plan with the team's Admin API key (documented: POST
// https://api.cursor.com/teams/spend, Basic auth with the key as user
// name). Individual plans have no key-based API and are not read
// (SPECS §3).
package cursoradminapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.cursor.com"

const spendPath = "/teams/spend"

const maxCents = 1e11

// Spend is one member's spending in the current cycle, in US dollars:
// OnDemand is what is billed beyond the plan, Overall adds the included
// usage; Limit is the member's on-demand limit (0 when none).
type Spend struct {
	OnDemand   float64
	Overall    float64
	Limit      float64
	CycleStart time.Time
}

// ErrNoMember is an answer without the configured member.
var ErrNoMember = errors.New("no team member has that email")

// Source reads one member of a Cursor team.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	email    string
	now      func() time.Time
}

// New reads the member with that email, with the named Admin API key.
func New(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, email string, now func() time.Time) Source {
	return Source{endpoint: endpoint, keys: keys, key: key, email: email, now: now}
}

// Read asks for the member's spending in the current cycle.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	query, _ := json.Marshal(map[string]any{"searchTerm": s.email, "pageSize": 100})
	body, err := providerhttp.PostBasic(ctx, s.endpoint, spendPath, key, query, now)
	if err != nil {
		return domain.Reading{}, err
	}
	spend, err := DecodeSpend(body, s.email)
	if errors.Is(err, ErrNoMember) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, ErrNoMember.Error()+": check \"email\" in config.json", nil)
	}
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now,
	}
	spent, err := providerhttp.Dollars(spend.Overall)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	if spend.Limit <= 0 {
		balance, _ := domain.NewBalance(nil, &spent)
		facts.Balance, facts.Outcome = &balance, domain.OutcomePartial
		facts.Hint = "no on-demand limit is set for this member: only the spending this cycle is known"
		return domain.NewReading(facts)
	}
	used, err := domain.NewUsedPercent(min(spend.OnDemand/spend.Limit*100, 10000))
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	facts.Meters = []domain.Meter{domain.NewMeter("on_demand", domain.BillingCycleWindow(), used, NextCycle(spend.CycleStart, now))}
	left, err := providerhttp.Dollars(max(spend.Limit-spend.OnDemand, 0))
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	balance, _ := domain.NewBalance(&left, &spent)
	facts.Balance = &balance
	return domain.NewReading(facts)
}

// NextCycle is the first monthly anniversary of the cycle start after now;
// zero when the start is unknown.
func NextCycle(start, now time.Time) time.Time {
	if start.IsZero() {
		return time.Time{}
	}
	next := start
	for months := 1; !next.After(now) && months <= 1200; months++ {
		next = start.AddDate(0, months, 0)
	}
	return next
}

type memberDocument struct {
	Email                        string   `json:"email"`
	SpendCents                   *float64 `json:"spendCents"`
	OverallSpendCents            *float64 `json:"overallSpendCents"`
	MonthlyLimitDollars          *float64 `json:"monthlyLimitDollars"`
	HardLimitOverrideDollars     *float64 `json:"hardLimitOverrideDollars"`
	EffectivePerUserLimitDollars *float64 `json:"effectivePerUserLimitDollars"`
}

// DecodeSpend finds the member by email (any case) in teamMemberSpend[]
// and reads its spending in cents and its limit in dollars: the effective
// per-user limit, else the member's override, else the monthly limit.
func DecodeSpend(body []byte, email string) (Spend, error) {
	var document struct {
		Members    *[]memberDocument `json:"teamMemberSpend"`
		CycleStart *float64          `json:"subscriptionCycleStart"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Spend{}, err
	}
	if document.Members == nil {
		return Spend{}, errors.New("teamMemberSpend is required")
	}
	for _, member := range *document.Members {
		if !strings.EqualFold(strings.TrimSpace(member.Email), email) {
			continue
		}
		spend, err := decodeMember(member)
		if err != nil {
			return Spend{}, err
		}
		if document.CycleStart != nil && *document.CycleStart > 0 {
			if *document.CycleStart < 1e12 || *document.CycleStart > 1e14 {
				return Spend{}, errors.New("subscriptionCycleStart is not in epoch milliseconds")
			}
			spend.CycleStart = time.UnixMilli(int64(*document.CycleStart)).UTC()
		}
		return spend, nil
	}
	return Spend{}, ErrNoMember
}

func decodeMember(member memberDocument) (Spend, error) {
	if member.SpendCents == nil || member.OverallSpendCents == nil {
		return Spend{}, errors.New("a member needs spendCents and overallSpendCents")
	}
	for _, cents := range []float64{*member.SpendCents, *member.OverallSpendCents} {
		if cents < 0 || cents > maxCents {
			return Spend{}, errors.New("spending is out of range")
		}
	}
	spend := Spend{OnDemand: *member.SpendCents / 100, Overall: *member.OverallSpendCents / 100}
	for _, limit := range []*float64{member.EffectivePerUserLimitDollars, member.HardLimitOverrideDollars, member.MonthlyLimitDollars} {
		if limit != nil && *limit > 0 && *limit < maxCents/100 {
			spend.Limit = *limit
			break
		}
	}
	return spend, nil
}
