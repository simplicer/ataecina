package opencodeconsole

import (
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// keptLimits is all that is kept of an answer (officialcli.Reuse): the
// figures DecodeLimits extracted, or that OpenCode was signed out. The
// workspace directory and anything else in the answer are never kept.
type keptLimits struct {
	SignedOut bool         `json:"signed_out,omitempty"`
	Go        []keptWindow `json:"go,omitempty"`
	Budgets   []keptBudget `json:"budgets,omitempty"`
}

type keptWindow struct {
	Kind     string    `json:"kind"`
	Minutes  int       `json:"minutes"`
	Percent  float64   `json:"percent"`
	Limited  bool      `json:"limited,omitempty"`
	ResetsAt time.Time `json:"resets_at"`
}

type keptBudget struct {
	Limit    float64   `json:"limit_usd"`
	Spent    float64   `json:"spent_usd"`
	ResetsAt time.Time `json:"resets_at"`
}

func keep(limits Limits) keptLimits {
	var kept keptLimits
	for _, window := range limits.Go {
		kept.Go = append(kept.Go, keptWindow{
			Kind: string(window.Window.Kind()), Minutes: window.Window.Minutes(),
			Percent: window.Percent, Limited: window.Limited, ResetsAt: window.ResetsAt,
		})
	}
	for _, budget := range limits.Budgets {
		kept.Budgets = append(kept.Budgets, keptBudget(budget))
	}
	return kept
}

// limits restores what was kept; ErrSignedOut when OpenCode was signed out.
func (k keptLimits) limits() (Limits, error) {
	if k.SignedOut {
		return Limits{}, ErrSignedOut
	}
	var limits Limits
	for _, kept := range k.Go {
		window, err := domain.ParseWindow(kept.Kind, kept.Minutes)
		if err != nil {
			return Limits{}, err
		}
		limits.Go = append(limits.Go, GoWindow{Window: window, Percent: kept.Percent, Limited: kept.Limited, ResetsAt: kept.ResetsAt})
	}
	for _, budget := range k.Budgets {
		limits.Budgets = append(limits.Budgets, Budget(budget))
	}
	return limits, nil
}
