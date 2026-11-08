// Package moonshotapi reads the prepaid balance of the Moonshot AI (Kimi)
// open platform with the owner's platform key (documented: GET
// /v1/users/me/balance). The international platform bills in USD, the
// Chinese one in CNY; a key works only on its own platform.
package moonshotapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Platform is one of the two Moonshot platforms: its only host and the
// currency it bills in.
type Platform struct {
	Origin   string
	Currency string
}

// Global is platform.moonshot.ai; China is platform.moonshot.cn.
var (
	Global = Platform{Origin: "https://api.moonshot.ai", Currency: "USD"}
	China  = Platform{Origin: "https://api.moonshot.cn", Currency: "CNY"}
)

const balancePath = "/v1/users/me/balance"

const maxMoney = 1e9

// Balance is the platform's answer: what can be spent, the vouchers and
// the cash (negative when money is owed).
type Balance struct {
	Available float64
	Voucher   float64
	Cash      float64
}

// Source reads one Moonshot platform account.
type Source struct {
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	currency domain.Currency
	now      func() time.Time
}

// New reads with the named key through endpoint, in the platform's
// currency.
func New(platform Platform) func(httpclient.Endpoint, domain.KeyRing, domain.KeyName, func() time.Time) Source {
	return func(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, now func() time.Time) Source {
		currency, _ := domain.NewCurrency(platform.Currency)
		return Source{endpoint: endpoint, keys: keys, key: key, currency: currency, now: now}
	}
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
	amount, err := domain.AmountFromFloat(balance.Available)
	if err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	remaining := domain.NewMoney(amount, s.currency)
	money, _ := domain.NewBalance(&remaining, nil)
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now, Balance: &money,
	}
	if balance.Cash < 0 {
		facts.Hint = "the cash balance is negative: money is owed"
	}
	return domain.NewReading(facts)
}

// DecodeBalance reads data.{available_balance, voucher_balance,
// cash_balance}; `code` must be 0 and `status` true.
func DecodeBalance(body []byte) (Balance, error) {
	var document struct {
		Code   *int  `json:"code"`
		Status *bool `json:"status"`
		Data   *struct {
			Available *float64 `json:"available_balance"`
			Voucher   *float64 `json:"voucher_balance"`
			Cash      *float64 `json:"cash_balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Balance{}, err
	}
	if document.Code == nil || *document.Code != 0 || document.Status == nil || !*document.Status {
		return Balance{}, errors.New("code 0 and status true are required")
	}
	data := document.Data
	if data == nil || data.Available == nil || data.Voucher == nil || data.Cash == nil {
		return Balance{}, errors.New("data needs available_balance, voucher_balance and cash_balance")
	}
	for _, amount := range []float64{*data.Available, *data.Voucher, *data.Cash} {
		if amount < -maxMoney || amount > maxMoney {
			return Balance{}, errors.New("an amount is out of range")
		}
	}
	if *data.Voucher < 0 {
		return Balance{}, errors.New("a negative voucher balance")
	}
	return Balance{Available: *data.Available, Voucher: *data.Voucher, Cash: *data.Cash}, nil
}
