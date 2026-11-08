package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

// ErrNoRanking is a ranking that was never fetched.
var ErrNoRanking = errors.New("no intelligence ranking yet")

// Score is one model's place in an external intelligence ranking (the
// Artificial Analysis Intelligence Index): its name there and its index.
type Score struct {
	Name  string
	Index float64
}

// Ranking is the intelligence index of many models, the version of the
// index (empty when the source does not say), who passed it on (empty
// when read from Artificial Analysis itself, "OpenRouter" otherwise),
// and when it was fetched.
type Ranking struct {
	Version     string
	Via         string
	RefreshedAt time.Time
	Scores      []Score
}

var (
	parenthesised = regexp.MustCompile(`\([^)]*\)`)
	dateSuffix    = regexp.MustCompile(`-20\d{6}$`)
	notAlphanum   = regexp.MustCompile(`[^a-z0-9]+`)
)

// ModelKey is how a model is recognised across catalogues: lower case,
// without a vendor prefix ("z-ai/"), a free suffix (":free", "-free"), a
// release date ("-20260816"), variants in parentheses ("(Max)") or
// punctuation, so that "z-ai/glm-5.3:free", "z-ai/glm-5.3-20260816",
// "glm-5.3" and "GLM-5.3 (Max)" are one model.
func ModelKey(text string) string {
	key := strings.ToLower(strings.TrimSpace(text))
	if slash := strings.LastIndex(key, "/"); slash >= 0 {
		key = key[slash+1:]
	}
	key = strings.TrimSuffix(key, ":free")
	key = parenthesised.ReplaceAllString(key, " ")
	key = strings.Trim(notAlphanum.ReplaceAllString(key, "-"), "-")
	return dateSuffix.ReplaceAllString(strings.TrimSuffix(key, "-free"), "")
}

// Indexer finds offerings in a ranking by key.
type Indexer map[string]float64

// Indexer keys the ranking once; when it has several variants of a model
// (reasoning efforts), the best counts.
func (r Ranking) Indexer() Indexer {
	index := make(Indexer, len(r.Scores))
	for _, score := range r.Scores {
		key := ModelKey(score.Name)
		if known, ok := index[key]; key != "" && (!ok || score.Index > known) {
			index[key] = score.Index
		}
	}
	return index
}

// IndexOf finds an offering's index by its id, then by its name.
func (i Indexer) IndexOf(offering Offering) (float64, bool) {
	for _, key := range []string{ModelKey(offering.Model), ModelKey(offering.Name)} {
		if index, ok := i[key]; ok && key != "" {
			return index, true
		}
	}
	return 0, false
}

// RankingSource fetches the ranking.
type RankingSource interface {
	Fetch(ctx context.Context) (Ranking, error)
}

// Rankings keeps the ranking between updates.
type Rankings interface {
	Load() (Ranking, error)
	Save(Ranking) error
}
