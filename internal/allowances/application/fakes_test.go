package application_test

import (
	"context"
	"sync"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type sourceFunc func(ctx context.Context, subscription domain.Subscription) (domain.Reading, error)

func (f sourceFunc) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	return f(ctx, subscription)
}

type catalog map[domain.SubscriptionID]domain.Source

func (c catalog) SourceOf(subscription domain.Subscription) (domain.Source, bool) {
	source, ok := c[subscription.ID()]
	return source, ok
}

type snapshotShelf struct {
	previous   domain.Snapshot
	readErr    error
	kept       *domain.Snapshot
	keepErr    error
	keepCalled int
}

func (s *snapshotShelf) LatestSnapshot() (domain.Snapshot, error) {
	return s.previous, s.readErr
}

func (s *snapshotShelf) KeepSnapshot(snapshot domain.Snapshot) error {
	s.keepCalled++
	if s.keepErr != nil {
		return s.keepErr
	}
	s.kept = &snapshot
	return nil
}

type journal struct {
	mu      sync.Mutex
	records []domain.CollectionRecord
}

func (j *journal) Record(record domain.CollectionRecord) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = append(j.records, record)
}
