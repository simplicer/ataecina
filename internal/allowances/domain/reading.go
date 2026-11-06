package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidReading reports a reading that breaks the model's rules.
var ErrInvalidReading = errors.New("invalid reading")

// ReadingFacts are the raw facts of a reading, validated by NewReading.
type ReadingFacts struct {
	Subscription   SubscriptionID
	Provider       ProviderID
	Source         SourceID
	Plan           Plan
	Fidelity       Fidelity
	Outcome        Outcome
	ObservedAt     time.Time
	Meters         []Meter
	Balance        *Balance
	Credits        *Credits
	BankedResets   *BankedResets
	ErrorClass     ErrorClass
	Hint           string
	RetryNotBefore time.Time
}

// Reading is a dated observation of one subscription's allowance from one
// source, with its fidelity and outcome.
type Reading struct {
	facts ReadingFacts
}

// NewReading validates the facts of a reading.
func NewReading(facts ReadingFacts) (Reading, error) {
	if facts.Subscription == "" || facts.Provider == "" || facts.Source == "" {
		return Reading{}, fmt.Errorf("%w: subscription, provider and source are required", ErrInvalidReading)
	}
	if _, err := ParseFidelity(string(facts.Fidelity)); err != nil {
		return Reading{}, fmt.Errorf("%w: %w", ErrInvalidReading, err)
	}
	if _, err := ParseOutcome(string(facts.Outcome)); err != nil {
		return Reading{}, fmt.Errorf("%w: %w", ErrInvalidReading, err)
	}
	if _, err := ParseErrorClass(string(facts.ErrorClass)); err != nil {
		return Reading{}, fmt.Errorf("%w: %w", ErrInvalidReading, err)
	}
	hasFigures := len(facts.Meters) > 0 || facts.Balance != nil || facts.Credits != nil || facts.BankedResets != nil
	if facts.Outcome.HasFigures() && (!hasFigures || facts.ObservedAt.IsZero()) {
		return Reading{}, fmt.Errorf("%w: %s needs figures and an observation time", ErrInvalidReading, facts.Outcome)
	}
	if !facts.Outcome.HasFigures() && hasFigures {
		return Reading{}, fmt.Errorf("%w: %s cannot carry figures", ErrInvalidReading, facts.Outcome)
	}
	for i, meter := range facts.Meters {
		for _, earlier := range facts.Meters[:i] {
			if meter.sameSlot(earlier) {
				return Reading{}, fmt.Errorf("%w: two %s meters in bucket %q", ErrInvalidReading, meter.window.kind, meter.bucket)
			}
		}
	}
	facts.Meters = append([]Meter(nil), facts.Meters...)
	facts.ObservedAt = facts.ObservedAt.UTC()
	facts.RetryNotBefore = facts.RetryNotBefore.UTC()
	facts.Hint = cleanHint(facts.Hint)
	return Reading{facts: facts}, nil
}

// Subscription is the subscription read.
func (r Reading) Subscription() SubscriptionID { return r.facts.Subscription }

// Provider is the provider behind it.
func (r Reading) Provider() ProviderID { return r.facts.Provider }

// Source is where the reading came from.
func (r Reading) Source() SourceID { return r.facts.Source }

// Plan is the reported or declared plan; empty when unknown.
func (r Reading) Plan() Plan { return r.facts.Plan }

// Fidelity is how far the figures can be trusted.
func (r Reading) Fidelity() Fidelity { return r.facts.Fidelity }

// Outcome is the result of the latest attempt.
func (r Reading) Outcome() Outcome { return r.facts.Outcome }

// ObservedAt is when the figures were observed, if ever.
func (r Reading) ObservedAt() (time.Time, bool) {
	return r.facts.ObservedAt, !r.facts.ObservedAt.IsZero()
}

// Meters are the windowed allowances.
func (r Reading) Meters() []Meter { return append([]Meter(nil), r.facts.Meters...) }

// Balance is the prepaid money, when the subscription has one.
func (r Reading) Balance() (Balance, bool) {
	if r.facts.Balance == nil {
		return Balance{}, false
	}
	return *r.facts.Balance, true
}

// Credits are non-money credits, when reported.
func (r Reading) Credits() (Credits, bool) {
	if r.facts.Credits == nil {
		return Credits{}, false
	}
	return *r.facts.Credits, true
}

// BankedResets are the resets the owner may trigger early, when reported.
func (r Reading) BankedResets() (BankedResets, bool) {
	if r.facts.BankedResets == nil {
		return BankedResets{}, false
	}
	return *r.facts.BankedResets, true
}

// ErrorClass is why the latest attempt failed; empty when it did not.
func (r Reading) ErrorClass() ErrorClass { return r.facts.ErrorClass }

// Hint is a short owner-facing explanation.
func (r Reading) Hint() string { return r.facts.Hint }

// RetryNotBefore is when the source may be asked again, when it said so.
func (r Reading) RetryNotBefore() (time.Time, bool) {
	return r.facts.RetryNotBefore, !r.facts.RetryNotBefore.IsZero()
}

// WaitsAt reports whether the source asked not to be called before now.
func (r Reading) WaitsAt(now time.Time) bool { return now.Before(r.facts.RetryNotBefore) }

// Freshness is the reading's freshness at now.
func (r Reading) Freshness(now time.Time, interval time.Duration) Freshness {
	return FreshnessOf(r.facts.ObservedAt, now, interval)
}

// At is the reading as it stands at now, with reborn windows estimated.
func (r Reading) At(now time.Time) Reading {
	facts := r.facts
	facts.Meters = make([]Meter, len(r.facts.Meters))
	for i, meter := range r.facts.Meters {
		facts.Meters[i] = meter.At(now)
	}
	return Reading{facts: facts}
}

// WorstMeter is the most used meter, if any.
func (r Reading) WorstMeter() (Meter, bool) {
	var worst Meter
	found := false
	for _, meter := range r.facts.Meters {
		if !found || meter.used > worst.used {
			worst, found = meter, true
		}
	}
	return worst, found
}

// Settle decides what the snapshot keeps for a subscription after its
// source answered (fresh) or failed. A transient failure keeps the last
// good reading and records why it was not refreshed; other failures
// replace it (SPECS §4.3, ADR 0011).
func Settle(subscription Subscription, previous *Reading, fresh Reading, failure error) Reading {
	if failure == nil {
		if fresh.facts.Subscription != subscription.id {
			return failedReading(subscription, Failure(ErrorContractChanged, "the source answered for another subscription", nil))
		}
		if fresh.facts.Plan.IsUnknown() {
			fresh.facts.Plan = subscription.plan
		}
		fresh.facts.RetryNotBefore = time.Time{}
		return fresh
	}
	var classified *SourceFailure
	if !errors.As(failure, &classified) {
		classified = &SourceFailure{outcome: OutcomeBroken, hint: "unexpected failure while reading the source"}
	}
	if classified.class.IsTransient() && previous != nil && previous.facts.Outcome.HasFigures() {
		kept := *previous
		kept.facts.ErrorClass = classified.class
		kept.facts.Hint = classified.hint
		kept.facts.RetryNotBefore = classified.retryAt
		return kept
	}
	return failedReading(subscription, classified)
}

func failedReading(subscription Subscription, failure *SourceFailure) Reading {
	return Reading{facts: ReadingFacts{
		Subscription:   subscription.id,
		Provider:       subscription.provider,
		Source:         subscription.source,
		Plan:           subscription.plan,
		Fidelity:       subscription.fidelity,
		Outcome:        failure.outcome,
		ErrorClass:     failure.class,
		Hint:           failure.hint,
		RetryNotBefore: failure.retryAt,
	}}
}
