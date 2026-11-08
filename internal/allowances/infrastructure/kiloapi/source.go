// Package kiloapi reads the Kilo Gateway balance with the owner's Kilo API
// key (undocumented; answered by GET https://api.kilo.ai/api/profile/balance
// with the key as Bearer): `balance` (US dollars, to verify) and
// `isDepleted`.
package kiloapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://api.kilo.ai"

const balancePath = "/api/profile/balance"

// Balance is the account's credit and whether it is used up.
type Balance struct {
	Left     float64
	Depleted bool
}

// Source reads one Kilo Gateway account.
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

// Read asks for the account's balance.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	now := s.now()
	body, err := providerhttp.Fetch(ctx, s.endpoint, balancePath, key, now)
	if err != nil {
		return domain.Reading{}, err
	}
	balance, err := DecodeBalance(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	left, err := providerhttp.Dollars(balance.Left)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	money, _ := domain.NewBalance(&left, nil)
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Balance: &money,
	}
	if balance.Depleted {
		facts.Hint = "the Kilo balance is used up: only the free models answer"
	}
	return domain.NewReading(facts)
}

// DecodeBalance reads balance (a number) and isDepleted (a switch).
func DecodeBalance(body []byte) (Balance, error) {
	var document struct {
		Balance  *float64 `json:"balance"`
		Depleted *bool    `json:"isDepleted"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Balance{}, err
	}
	if document.Balance == nil || document.Depleted == nil {
		return Balance{}, errors.New("balance and isDepleted are required")
	}
	if *document.Balance < -1e9 || *document.Balance > 1e9 {
		return Balance{}, errors.New("balance is out of range")
	}
	return Balance{Left: *document.Balance, Depleted: *document.Depleted}, nil
}
