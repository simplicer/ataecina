package opencodezen

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// SourceID is the id of this source in config.json.
const SourceID domain.SourceID = "opencode-zen"

// Source derives the OpenCode Zen balance for the collector.
type Source struct {
	declarations domain.BalanceDeclarations
	spend        domain.RecordedSpend
	now          func() time.Time
}

// NewSource combines the owner's declarations with the recorded spending.
func NewSource(declarations domain.BalanceDeclarations, spend domain.RecordedSpend, now func() time.Time) Source {
	return Source{declarations: declarations, spend: spend, now: now}
}

// Read derives what is left. Without any declaration it shows the
// spending to date only, as PARTIAL, with a hint to declare the balance.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	spent, err := s.spend.SpentToDate(ctx)
	if err != nil {
		return domain.Reading{}, err
	}
	declarations, err := s.declarations.DeclarationsOf(subscription.ID())
	if err != nil {
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "the declared balances cannot be read", err)
	}
	facts := domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Fidelity: domain.FidelityDerived, Outcome: domain.OutcomeOK, ObservedAt: s.now(),
	}
	balance, err := domain.DeriveBalance(declarations, spent)
	switch {
	case errors.Is(err, domain.ErrNoDeclaration):
		spentOnly, _ := domain.NewBalance(nil, &spent)
		facts.Balance, facts.Outcome = &spentOnly, domain.OutcomePartial
		facts.Hint = fmt.Sprintf("declare the balance: ataecina balance declare %s --balance <amount>", subscription.ID())
	case err != nil:
		return domain.Reading{}, domain.Failure(domain.ErrorUnreadable, "the declared balances are in another currency than the spending", err)
	default:
		facts.Balance = &balance
	}
	return domain.NewReading(facts)
}
