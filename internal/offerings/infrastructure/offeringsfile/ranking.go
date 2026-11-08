package offeringsfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

const rankingMaxBytes = 4 << 20

type rankingDocument struct {
	Format      int             `json:"format"`
	Version     string          `json:"index_version"`
	Via         string          `json:"via,omitempty"`
	RefreshedAt time.Time       `json:"refreshed_at"`
	Scores      []scoreDocument `json:"scores"`
}

type scoreDocument struct {
	Name  string  `json:"name"`
	Index float64 `json:"index"`
}

// RankingStore keeps the intelligence ranking in
// $XDG_STATE_HOME/ataecina/intelligence.json (0600).
type RankingStore struct {
	dir string
}

// NewRankingStore keeps the ranking in dir.
func NewRankingStore(dir string) RankingStore { return RankingStore{dir: dir} }

func (s RankingStore) path() string { return filepath.Join(s.dir, "intelligence.json") }

// Load reads the kept ranking; a missing file is domain.ErrNoRanking.
func (s RankingStore) Load() (domain.Ranking, error) {
	data, err := privatefs.ReadFile(s.path(), rankingMaxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Ranking{}, domain.ErrNoRanking
	}
	if err != nil {
		return domain.Ranking{}, err
	}
	var stored rankingDocument
	if err := json.Unmarshal(data, &stored); err != nil {
		return domain.Ranking{}, fmt.Errorf("%s: %w", s.path(), err)
	}
	if stored.Format != formatVersion {
		return domain.Ranking{}, fmt.Errorf("%s: unknown format %d", s.path(), stored.Format)
	}
	ranking := domain.Ranking{Version: stored.Version, Via: stored.Via, RefreshedAt: stored.RefreshedAt}
	for _, score := range stored.Scores {
		ranking.Scores = append(ranking.Scores, domain.Score(score))
	}
	return ranking, nil
}

// Save replaces the kept ranking atomically.
func (s RankingStore) Save(ranking domain.Ranking) error {
	stored := rankingDocument{Format: formatVersion, Version: ranking.Version, Via: ranking.Via, RefreshedAt: ranking.RefreshedAt.UTC(), Scores: []scoreDocument{}}
	for _, score := range ranking.Scores {
		stored.Scores = append(stored.Scores, scoreDocument(score))
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	if err := privatefs.EnsureDir(s.dir); err != nil {
		return err
	}
	return privatefs.WriteFile(s.path(), data)
}
