// Package application holds the allowances use cases, one per file.
package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// CollectionPolicy is how often and how long the collector may run.
type CollectionPolicy struct {
	Interval time.Duration
	Deadline time.Duration
}

// CollectReadingsPorts are the ports the collector needs.
type CollectReadingsPorts struct {
	Sources  domain.SourceCatalog
	Previous domain.SnapshotReader
	Store    domain.SnapshotWriter
	Journal  domain.CollectionJournal
	Clock    domain.Clock
}

// CollectReadings asks every configured subscription's source once,
// concurrently and within a global deadline, settles each answer against
// the previous snapshot and keeps the new snapshot (SPECS §5).
type CollectReadings struct {
	subscriptions []domain.Subscription
	policy        CollectionPolicy
	ports         CollectReadingsPorts
}

// NewCollectReadings prepares the collector for the configured subscriptions.
func NewCollectReadings(subscriptions []domain.Subscription, policy CollectionPolicy, ports CollectReadingsPorts) CollectReadings {
	return CollectReadings{subscriptions: append([]domain.Subscription(nil), subscriptions...), policy: policy, ports: ports}
}

type collected struct {
	reading domain.Reading
	latency time.Duration
	asked   bool
}

// Collect reads every subscription and keeps the resulting snapshot.
func (c CollectReadings) Collect(ctx context.Context) (domain.Snapshot, error) {
	previous, err := c.ports.Previous.LatestSnapshot()
	if err != nil {
		// A missing or unreadable snapshot only means there is nothing to keep.
		previous = domain.Snapshot{}
	}
	now := c.ports.Clock.Now()
	ctx, cancel := context.WithTimeout(ctx, c.policy.Deadline)
	defer cancel()

	results := make([]collected, len(c.subscriptions))
	var wg sync.WaitGroup
	for i, subscription := range c.subscriptions {
		last, hasLast := previous.ReadingOf(subscription.ID())
		var lastPointer *domain.Reading
		if hasLast {
			lastPointer = &last
		}
		if hasLast && last.WaitsAt(now) {
			results[i] = collected{reading: last}
			continue
		}
		source, found := c.ports.Sources.SourceOf(subscription)
		if !found {
			unsupported := domain.Unsupported(fmt.Sprintf("source %s is not available in this version", subscription.Source()))
			results[i] = collected{reading: domain.Settle(subscription, lastPointer, domain.Reading{}, unsupported)}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := c.ports.Clock.Now()
			fresh, failure := ask(ctx, source, subscription)
			results[i] = collected{
				reading: domain.Settle(subscription, lastPointer, fresh, failure),
				latency: c.ports.Clock.Now().Sub(started),
				asked:   true,
			}
		}()
	}
	wg.Wait()

	readings := make([]domain.Reading, len(results))
	for i, result := range results {
		readings[i] = result.reading
	}
	snapshot, err := domain.NewSnapshot(now, c.policy.Interval, readings)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("collect readings: %w", err)
	}
	if err := c.ports.Store.KeepSnapshot(snapshot); err != nil {
		return domain.Snapshot{}, fmt.Errorf("keep snapshot: %w", err)
	}
	for _, result := range results {
		c.ports.Journal.Record(domain.CollectionRecord{
			Reading:   result.reading,
			Freshness: result.reading.Freshness(now, c.policy.Interval),
			Latency:   result.latency,
			Asked:     result.asked,
		})
	}
	return snapshot, nil
}

// ask runs one source and gives up at the deadline even if the source
// ignores its context; a panicking adapter breaks only its subscription.
func ask(ctx context.Context, source domain.Source, subscription domain.Subscription) (domain.Reading, error) {
	type answer struct {
		reading domain.Reading
		err     error
	}
	answers := make(chan answer, 1)
	go func() {
		defer func() {
			if recover() != nil {
				answers <- answer{err: errors.New("the source adapter failed unexpectedly")}
			}
		}()
		reading, err := source.Read(ctx, subscription)
		answers <- answer{reading, err}
	}()
	select {
	case a := <-answers:
		return a.reading, a.err
	case <-ctx.Done():
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "no answer before the collection deadline", ctx.Err())
	}
}
