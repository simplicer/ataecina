package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// DeclareBalance records a top-up or the current balance of a balance-only
// subscription, together with the money spent to date, so the collector
// can derive what is left (SPECS §3, `opencode-zen`).
type DeclareBalance struct {
	declarations domain.BalanceDeclarations
	spend        domain.RecordedSpend
	clock        domain.Clock
}

// NewDeclareBalance prepares the declaration of one subscription's balance.
func NewDeclareBalance(declarations domain.BalanceDeclarations, spend domain.RecordedSpend, clock domain.Clock) DeclareBalance {
	return DeclareBalance{declarations: declarations, spend: spend, clock: clock}
}

// Declare records the declaration and returns it.
func (d DeclareBalance) Declare(ctx context.Context, subscription domain.SubscriptionID, kind domain.DeclarationKind, amount domain.Money) (domain.Declaration, error) {
	spent, err := d.spend.SpentToDate(ctx)
	if err != nil {
		return domain.Declaration{}, fmt.Errorf("declare balance: the spending to date cannot be read: %w", err)
	}
	declaration, err := domain.NewDeclaration(kind, amount, d.clock.Now(), spent)
	if err != nil {
		return domain.Declaration{}, fmt.Errorf("declare balance: %w", err)
	}
	previous, err := d.declarations.DeclarationsOf(subscription)
	if err != nil {
		return domain.Declaration{}, fmt.Errorf("declare balance: %w", err)
	}
	if _, err := domain.DeriveBalance(append(previous, declaration), spent); err != nil {
		return domain.Declaration{}, fmt.Errorf("declare balance: %w", err)
	}
	if err := d.declarations.KeepDeclaration(subscription, declaration); err != nil {
		return domain.Declaration{}, fmt.Errorf("declare balance: %w", err)
	}
	return declaration, nil
}
