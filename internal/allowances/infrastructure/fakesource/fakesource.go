// Package fakesource is a provider that answers from JSON files in a
// directory. It is linked only into binaries built with the e2e tag, so
// end-to-end tests can drive every outcome without a real provider.
package fakesource

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

type answer struct {
	Failure            string  `json:"failure"`
	Plan               string  `json:"plan"`
	ObservedMinutesAgo int     `json:"observed_minutes_ago"`
	Meters             []meter `json:"meters"`
	Balance            *struct {
		Remaining string `json:"remaining"`
		Spent     string `json:"spent"`
		Currency  string `json:"currency"`
	} `json:"balance"`
}

type meter struct {
	Bucket          string  `json:"bucket"`
	WindowMinutes   int     `json:"window_minutes"`
	UsedPercent     float64 `json:"used_percent"`
	ResetsInMinutes int     `json:"resets_in_minutes"`
}

// Source reads <dir>/<subscription>.json.
type Source struct {
	dir string
}

// New answers from dir.
func New(dir string) Source { return Source{dir: dir} }

// Read builds the reading described by the subscription's file.
func (s Source) Read(_ context.Context, subscription domain.Subscription) (domain.Reading, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, subscription.ID().String()+".json"))
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "no fake answer for this subscription", err)
	}
	var fake answer
	if err := json.Unmarshal(data, &fake); err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "fake answer is not JSON", err)
	}
	if fake.Failure == "UNSUPPORTED" {
		return domain.Reading{}, domain.Unsupported("fake unsupported source")
	}
	if fake.Failure != "" {
		return domain.Reading{}, domain.Failure(domain.ErrorClass(fake.Failure), "fake "+fake.Failure, nil)
	}
	now := time.Now().UTC()
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: domain.NewPlan(fake.Plan), Fidelity: subscription.Fidelity(), Outcome: domain.OutcomeOK,
		ObservedAt: now.Add(-time.Duration(fake.ObservedMinutesAgo) * time.Minute),
	}
	for _, m := range fake.Meters {
		window, err := domain.WindowOfMinutes(m.WindowMinutes)
		if err != nil {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "fake window", err)
		}
		used, err := domain.NewUsedPercent(m.UsedPercent)
		if err != nil {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "fake percentage", err)
		}
		var resetsAt time.Time
		if m.ResetsInMinutes != 0 {
			resetsAt = now.Add(time.Duration(m.ResetsInMinutes) * time.Minute)
		}
		facts.Meters = append(facts.Meters, domain.NewMeter(domain.NewBucketID(m.Bucket), window, used, resetsAt))
	}
	if fake.Balance != nil {
		balance, err := fakeBalance(fake.Balance.Remaining, fake.Balance.Spent, fake.Balance.Currency)
		if err != nil {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "fake balance", err)
		}
		facts.Balance = &balance
	}
	reading, err := domain.NewReading(facts)
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "fake reading", err)
	}
	return reading, nil
}

func fakeBalance(remaining, spent, code string) (domain.Balance, error) {
	currency, err := domain.NewCurrency(code)
	if err != nil {
		return domain.Balance{}, err
	}
	left, err := fakeMoney(remaining, currency)
	if err != nil {
		return domain.Balance{}, err
	}
	used, err := fakeMoney(spent, currency)
	if err != nil {
		return domain.Balance{}, err
	}
	return domain.NewBalance(left, used)
}

// fakeMoney is nil for an empty amount.
func fakeMoney(raw string, currency domain.Currency) (*domain.Money, error) {
	if raw == "" {
		return nil, nil
	}
	amount, err := domain.ParseAmount(raw)
	if err != nil {
		return nil, err
	}
	money := domain.NewMoney(amount, currency)
	return &money, nil
}
