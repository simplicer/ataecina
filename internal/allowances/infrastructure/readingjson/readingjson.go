// Package readingjson maps allowance figures to and from Ataecina's own
// JSON documents (the snapshot, hook signals and the public status
// contract): snake_case names, UPPER_SNAKE_CASE enums, RFC 3339 UTC
// instants and money as {amount, currency}.
package readingjson

import (
	"encoding/json"
	"fmt"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// Instant is an RFC 3339 date-time in UTC, to the second.
type Instant struct{ time.Time }

// InstantOf wraps a time; a zero time becomes nil (JSON null).
func InstantOf(t time.Time, known bool) *Instant {
	if !known || t.IsZero() {
		return nil
	}
	return &Instant{t.UTC()}
}

// MarshalJSON writes "2026-11-06T19:40:00Z".
func (i Instant) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.UTC().Format(time.RFC3339))
}

// UnmarshalJSON reads any RFC 3339 date-time.
func (i *Instant) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return err
	}
	i.Time = parsed.UTC()
	return nil
}

// Moment unwraps an optional instant.
func Moment(i *Instant) time.Time {
	if i == nil {
		return time.Time{}
	}
	return i.Time
}

// Meter is one windowed allowance.
type Meter struct {
	Bucket        *string  `json:"bucket"`
	Window        string   `json:"window"`
	WindowMinutes *int     `json:"window_minutes"`
	UsedPercent   float64  `json:"used_percent"`
	ResetsAt      *Instant `json:"resets_at"`
	Estimated     bool     `json:"estimated"`
}

// Money is an amount in an ISO 4217 currency.
type Money struct {
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
}

// Balance is prepaid money left and spent.
type Balance struct {
	Remaining *Money `json:"remaining"`
	Spent     *Money `json:"spent"`
}

// Credits are non-money provider credits.
type Credits struct {
	Available json.Number `json:"available"`
	Unlimited bool        `json:"unlimited"`
}

// BankedResets are resets the owner may trigger early.
type BankedResets struct {
	Available int `json:"available"`
}

// Reading is a full reading, as kept in the snapshot and in signals.
type Reading struct {
	Subscription   string        `json:"subscription"`
	Provider       string        `json:"provider"`
	Source         string        `json:"source"`
	Plan           *string       `json:"plan"`
	Fidelity       string        `json:"fidelity"`
	Outcome        string        `json:"outcome"`
	ObservedAt     *Instant      `json:"observed_at"`
	Meters         []Meter       `json:"meters"`
	Balance        *Balance      `json:"balance"`
	Credits        *Credits      `json:"credits"`
	BankedResets   *BankedResets `json:"banked_resets"`
	ErrorClass     *string       `json:"error_class"`
	Hint           *string       `json:"hint"`
	RetryNotBefore *Instant      `json:"retry_not_before,omitempty"`
}

// EncodeReading turns a reading into its document.
func EncodeReading(reading domain.Reading) Reading {
	observedAt, observed := reading.ObservedAt()
	retry, waits := reading.RetryNotBefore()
	return Reading{
		Subscription:   reading.Subscription().String(),
		Provider:       reading.Provider().String(),
		Source:         reading.Source().String(),
		Plan:           optional(reading.Plan().String()),
		Fidelity:       string(reading.Fidelity()),
		Outcome:        string(reading.Outcome()),
		ObservedAt:     InstantOf(observedAt, observed),
		Meters:         EncodeMeters(reading.Meters()),
		Balance:        EncodeBalance(reading.Balance()),
		Credits:        EncodeCredits(reading.Credits()),
		BankedResets:   EncodeBankedResets(reading.BankedResets()),
		ErrorClass:     optional(string(reading.ErrorClass())),
		Hint:           optional(reading.Hint()),
		RetryNotBefore: InstantOf(retry, waits),
	}
}

// DecodeReading validates a document back into a reading.
func DecodeReading(document Reading) (domain.Reading, error) {
	subscription, err := domain.NewSubscriptionID(document.Subscription)
	if err != nil {
		return domain.Reading{}, err
	}
	provider, err := domain.NewProviderID(document.Provider)
	if err != nil {
		return domain.Reading{}, err
	}
	source, err := domain.NewSourceID(document.Source)
	if err != nil {
		return domain.Reading{}, err
	}
	meters, err := DecodeMeters(document.Meters)
	if err != nil {
		return domain.Reading{}, err
	}
	balance, err := DecodeBalance(document.Balance)
	if err != nil {
		return domain.Reading{}, err
	}
	credits, err := DecodeCredits(document.Credits)
	if err != nil {
		return domain.Reading{}, err
	}
	banked, err := DecodeBankedResets(document.BankedResets)
	if err != nil {
		return domain.Reading{}, err
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription:   subscription,
		Provider:       provider,
		Source:         source,
		Plan:           domain.NewPlan(value(document.Plan)),
		Fidelity:       domain.Fidelity(document.Fidelity),
		Outcome:        domain.Outcome(document.Outcome),
		ObservedAt:     Moment(document.ObservedAt),
		Meters:         meters,
		Balance:        balance,
		Credits:        credits,
		BankedResets:   banked,
		ErrorClass:     domain.ErrorClass(value(document.ErrorClass)),
		Hint:           value(document.Hint),
		RetryNotBefore: Moment(document.RetryNotBefore),
	})
}

// EncodeMeters never returns nil, so an empty list is [] in JSON.
func EncodeMeters(meters []domain.Meter) []Meter {
	documents := make([]Meter, 0, len(meters))
	for _, meter := range meters {
		resetsAt, known := meter.ResetsAt()
		var minutes *int
		if m := meter.Window().Minutes(); m > 0 {
			minutes = &m
		}
		documents = append(documents, Meter{
			Bucket:        optional(meter.Bucket().String()),
			Window:        string(meter.Window().Kind()),
			WindowMinutes: minutes,
			UsedPercent:   meter.Used().Float64(),
			ResetsAt:      InstantOf(resetsAt, known),
			Estimated:     meter.Estimated(),
		})
	}
	return documents
}

// DecodeMeters validates meter documents.
func DecodeMeters(documents []Meter) ([]domain.Meter, error) {
	meters := make([]domain.Meter, 0, len(documents))
	for _, document := range documents {
		window, err := domain.ParseWindow(document.Window, value(document.WindowMinutes))
		if err != nil {
			return nil, err
		}
		used, err := domain.NewUsedPercent(document.UsedPercent)
		if err != nil {
			return nil, err
		}
		meters = append(meters, domain.NewMeter(domain.NewBucketID(value(document.Bucket)), window, used, Moment(document.ResetsAt)))
	}
	return meters, nil
}

// EncodeBalance writes a balance, or nil when there is none.
func EncodeBalance(balance domain.Balance, present bool) *Balance {
	if !present {
		return nil
	}
	document := &Balance{}
	if remaining, ok := balance.Remaining(); ok {
		document.Remaining = EncodeMoney(remaining)
	}
	if spent, ok := balance.Spent(); ok {
		document.Spent = EncodeMoney(spent)
	}
	return document
}

// DecodeBalance validates a balance document.
func DecodeBalance(document *Balance) (*domain.Balance, error) {
	if document == nil {
		return nil, nil
	}
	remaining, err := decodeMoney(document.Remaining)
	if err != nil {
		return nil, err
	}
	spent, err := decodeMoney(document.Spent)
	if err != nil {
		return nil, err
	}
	balance, err := domain.NewBalance(remaining, spent)
	if err != nil {
		return nil, err
	}
	return &balance, nil
}

// EncodeMoney writes {amount, currency}.
func EncodeMoney(money domain.Money) *Money {
	return &Money{Amount: json.Number(money.Amount().String()), Currency: money.Currency().String()}
}

func decodeMoney(document *Money) (*domain.Money, error) {
	if document == nil {
		return nil, nil
	}
	amount, err := domain.ParseAmount(document.Amount.String())
	if err != nil {
		return nil, err
	}
	currency, err := domain.NewCurrency(document.Currency)
	if err != nil {
		return nil, err
	}
	money := domain.NewMoney(amount, currency)
	return &money, nil
}

// EncodeCredits writes credits, or nil when there are none.
func EncodeCredits(credits domain.Credits, present bool) *Credits {
	if !present {
		return nil
	}
	return &Credits{Available: json.Number(credits.Available().String()), Unlimited: credits.Unlimited()}
}

// DecodeCredits validates a credits document.
func DecodeCredits(document *Credits) (*domain.Credits, error) {
	if document == nil {
		return nil, nil
	}
	available, err := domain.ParseAmount(document.Available.String())
	if err != nil {
		return nil, err
	}
	credits := domain.NewCredits(available, document.Unlimited)
	return &credits, nil
}

// EncodeBankedResets writes banked resets, or nil when unknown.
func EncodeBankedResets(banked domain.BankedResets, present bool) *BankedResets {
	if !present {
		return nil
	}
	return &BankedResets{Available: banked.Available()}
}

// DecodeBankedResets validates a banked resets document.
func DecodeBankedResets(document *BankedResets) (*domain.BankedResets, error) {
	if document == nil {
		return nil, nil
	}
	banked, err := domain.NewBankedResets(document.Available)
	if err != nil {
		return nil, fmt.Errorf("banked resets: %w", err)
	}
	return &banked, nil
}

func optional(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}

func value[T any](pointer *T) T {
	var zero T
	if pointer == nil {
		return zero
	}
	return *pointer
}
