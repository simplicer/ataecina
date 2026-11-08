// Package ollamaapi reads Ollama Cloud with the owner's API key through the
// documented balance endpoint (SPECS §3.3): GET /api/balance gives, for
// credit plans, the included USD left, the allowance and the period it
// renews at; for legacy plans, the share left of the session and weekly
// windows; and any purchased credit.
package ollamaapi

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
const Origin = "https://ollama.com"

const balancePath = "/api/balance"

const maxMoney = 1e9

// Included is the plan's credit for the current period, in USD.
type Included struct {
	Left      float64
	Allowance float64
	Until     time.Time
}

// Window is a legacy plan's window: the share left and when it resets
// (zero when Ollama gives no instant).
type Window struct {
	LeftPercent float64
	ResetsAt    time.Time
}

// Balance is what /api/balance reports; any part may be missing.
type Balance struct {
	Included  *Included
	Session   *Window
	Weekly    *Window
	Purchased *float64
}

// Source reads one Ollama Cloud subscription.
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

// Read asks for the balance of the key's account.
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
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: now,
	}
	if err := balance.describe(&facts); err != nil {
		return domain.Reading{}, providerhttp.Unexpected(s.endpoint.Host(), err)
	}
	return domain.NewReading(facts)
}

// describe turns the balance into meters and money: the included credit is
// a billing-cycle meter reborn at the period's end, legacy windows are
// five-hour and weekly meters, and the money left adds any purchased
// credit to the included one.
func (b Balance) describe(facts *domain.ReadingFacts) error {
	for _, window := range []struct {
		window domain.Window
		left   *Window
	}{{domain.FiveHourWindow(), b.Session}, {domain.WeeklyWindow(), b.Weekly}} {
		if window.left == nil {
			continue
		}
		used, err := domain.NewUsedPercent(100 - window.left.LeftPercent)
		if err != nil {
			return err
		}
		facts.Meters = append(facts.Meters, domain.NewMeter("", window.window, used, window.left.ResetsAt))
	}
	var left, spent *float64
	if b.Included != nil {
		used := 0.0
		if b.Included.Allowance > 0 {
			used = max(b.Included.Allowance-b.Included.Left, 0) / b.Included.Allowance * 100
		}
		percent, err := domain.NewUsedPercent(used)
		if err != nil {
			return err
		}
		facts.Meters = append(facts.Meters, domain.NewMeter("", domain.BillingCycleWindow(), percent, b.Included.Until))
		included, consumed := b.Included.Left, max(b.Included.Allowance-b.Included.Left, 0)
		left, spent = &included, &consumed
	}
	if b.Purchased != nil {
		total := *b.Purchased
		if left != nil {
			total += *left
		}
		left = &total
	}
	if left == nil {
		return nil
	}
	return addBalance(facts, *left, spent)
}

func addBalance(facts *domain.ReadingFacts, left float64, spent *float64) error {
	remaining, err := providerhttp.Dollars(left)
	if err != nil {
		return err
	}
	var spentMoney *domain.Money
	if spent != nil {
		money, err := providerhttp.Dollars(*spent)
		if err != nil {
			return err
		}
		spentMoney = &money
	}
	balance, err := domain.NewBalance(&remaining, spentMoney)
	if err != nil {
		return err
	}
	facts.Balance = &balance
	return nil
}

type periodDocument struct {
	Until *string `json:"until"`
}

type windowDocument struct {
	RemainingPercent *float64 `json:"remaining_percent"`
	ResetsAt         *string  `json:"resets_at"`
}

// DecodeBalance reads included.{balance_usd, allowance_usd, period.until},
// the legacy included.{session, weekly}.{remaining_percent, resets_at} and
// purchased.balance_usd; at least one figure is required.
func DecodeBalance(body []byte) (Balance, error) {
	var document struct {
		Included *struct {
			BalanceUSD   *float64        `json:"balance_usd"`
			AllowanceUSD *float64        `json:"allowance_usd"`
			Period       *periodDocument `json:"period"`
			Session      *windowDocument `json:"session"`
			Weekly       *windowDocument `json:"weekly"`
		} `json:"included"`
		Purchased *struct {
			BalanceUSD *float64 `json:"balance_usd"`
		} `json:"purchased"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Balance{}, err
	}
	var balance Balance
	if included := document.Included; included != nil {
		credit, err := decodeIncluded(included.BalanceUSD, included.AllowanceUSD, included.Period)
		if err != nil {
			return Balance{}, err
		}
		balance.Included = credit
		if balance.Session, err = decodeWindow(included.Session); err != nil {
			return Balance{}, err
		}
		if balance.Weekly, err = decodeWindow(included.Weekly); err != nil {
			return Balance{}, err
		}
	}
	if purchased := document.Purchased; purchased != nil && purchased.BalanceUSD != nil {
		if !validMoney(*purchased.BalanceUSD) {
			return Balance{}, errors.New("purchased.balance_usd is out of range")
		}
		balance.Purchased = purchased.BalanceUSD
	}
	if balance.Included == nil && balance.Session == nil && balance.Weekly == nil && balance.Purchased == nil {
		return Balance{}, errors.New("no included credit, window or purchased credit")
	}
	return balance, nil
}

func decodeIncluded(left, allowance *float64, period *periodDocument) (*Included, error) {
	if left == nil && allowance == nil {
		return nil, nil
	}
	if left == nil || allowance == nil || !validMoney(*left) || !validMoney(*allowance) {
		return nil, errors.New("included needs balance_usd and allowance_usd in range")
	}
	credit := &Included{Left: *left, Allowance: *allowance}
	if period != nil && period.Until != nil {
		until, err := time.Parse(time.RFC3339Nano, *period.Until)
		if err != nil {
			return nil, err
		}
		credit.Until = until.UTC()
	}
	return credit, nil
}

func decodeWindow(document *windowDocument) (*Window, error) {
	if document == nil {
		return nil, nil
	}
	if document.RemainingPercent == nil || *document.RemainingPercent < 0 || *document.RemainingPercent > 100 {
		return nil, errors.New("a window needs remaining_percent within 0–100")
	}
	window := &Window{LeftPercent: *document.RemainingPercent}
	if document.ResetsAt != nil {
		resetsAt, err := time.Parse(time.RFC3339Nano, *document.ResetsAt)
		if err != nil {
			return nil, err
		}
		window.ResetsAt = resetsAt.UTC()
	}
	return window, nil
}

func validMoney(value float64) bool {
	return value >= 0 && value <= maxMoney
}
