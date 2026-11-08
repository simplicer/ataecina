package application

import (
	"context"
	"fmt"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
)

// RefreshRanking fetches the intelligence ranking again, only when the
// owner asks (`ataecina update`): the index changes when models change,
// not every day (SPECS §11).
type RefreshRanking struct {
	source   domain.RankingSource
	rankings domain.Rankings
	clock    domain.Clock
}

// NewRefreshRanking prepares one refresh.
func NewRefreshRanking(source domain.RankingSource, rankings domain.Rankings, clock domain.Clock) RefreshRanking {
	return RefreshRanking{source: source, rankings: rankings, clock: clock}
}

// Refresh keeps the new ranking; a failure keeps the one already kept.
func (r RefreshRanking) Refresh(ctx context.Context) (domain.Ranking, error) {
	ranking, err := r.source.Fetch(ctx)
	if err != nil {
		return domain.Ranking{}, fmt.Errorf("refresh the intelligence ranking: %w", err)
	}
	ranking.RefreshedAt = r.clock.Now()
	if err := r.rankings.Save(ranking); err != nil {
		return domain.Ranking{}, fmt.Errorf("keep the intelligence ranking: %w", err)
	}
	return ranking, nil
}
