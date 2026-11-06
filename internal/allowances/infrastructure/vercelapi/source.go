// Package vercelapi reads the Vercel AI Gateway credit balance with the
// owner's API key (documented: GET https://ai-gateway.vercel.sh/v1/credits).
package vercelapi

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
const Origin = "https://ai-gateway.vercel.sh"

const creditsPath = "/v1/credits"

// Source reads one Vercel subscription.
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

// Read asks for the balance and the total spent.
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
	balance, err := DecodeCredits(body)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Balance: &balance,
	})
}

// DecodeCredits reads {"balance": "95.50", "total_used": "4.50"} (USD;
// strings in the documentation, numbers accepted too).
func DecodeCredits(body []byte) (domain.Balance, error) {
	var document struct {
		Balance   *json.Number `json:"balance"`
		TotalUsed *json.Number `json:"total_used"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return domain.Balance{}, err
	}
	if document.Balance == nil || document.TotalUsed == nil {
		return domain.Balance{}, errors.New("balance and total_used are required")
	}
	remaining, err := dollars(*document.Balance)
	if err != nil {
		return domain.Balance{}, err
	}
	spent, err := dollars(*document.TotalUsed)
	if err != nil {
		return domain.Balance{}, err
	}
	return domain.NewBalance(&remaining, &spent)
}

func dollars(number json.Number) (domain.Money, error) {
	amount, err := domain.ParseAmount(number.String())
	if err != nil {
		return domain.Money{}, err
	}
	return domain.NewMoney(amount, providerhttp.USD()), nil
}
