// Package domain holds the allowances model: subscriptions, meters,
// balances, readings, their fidelity and freshness, and the snapshot.
// It is pure: no I/O, no clock reads, no provider payloads.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidIdentifier reports a malformed subscription, provider or source id.
var ErrInvalidIdentifier = errors.New("invalid identifier")

var (
	shortIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	plainCharacters = regexp.MustCompile(`[^A-Z0-9]+`)
	bucketJunk      = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// SubscriptionID is the owner-chosen name of a subscription (claude, codex-work).
type SubscriptionID string

// NewSubscriptionID validates an owner-chosen subscription id.
func NewSubscriptionID(raw string) (SubscriptionID, error) {
	if !shortIdentifier.MatchString(raw) {
		return "", fmt.Errorf("%w: subscription %q (lowercase letters, digits and dashes, up to 32)", ErrInvalidIdentifier, raw)
	}
	return SubscriptionID(raw), nil
}

func (id SubscriptionID) String() string { return string(id) }

// ProviderID names the company or service behind a subscription (anthropic, openai).
type ProviderID string

// NewProviderID validates a provider id.
func NewProviderID(raw string) (ProviderID, error) {
	if !shortIdentifier.MatchString(raw) {
		return "", fmt.Errorf("%w: provider %q", ErrInvalidIdentifier, raw)
	}
	return ProviderID(raw), nil
}

func (id ProviderID) String() string { return string(id) }

// SourceID names where a reading comes from (codex-logs, zai-api).
type SourceID string

// NewSourceID validates a source id.
func NewSourceID(raw string) (SourceID, error) {
	if !shortIdentifier.MatchString(raw) {
		return "", fmt.Errorf("%w: source %q", ErrInvalidIdentifier, raw)
	}
	return SourceID(raw), nil
}

func (id SourceID) String() string { return string(id) }

// Label is the source as an UPPER_SNAKE_CASE value (CODEX_LOGS).
func (id SourceID) Label() string {
	return strings.ToUpper(strings.ReplaceAll(string(id), "-", "_"))
}

// Plan is a subscription plan (PRO, PLUS, LITE) in UPPER_SNAKE_CASE; empty
// when unknown.
type Plan string

const maxPlanLength = 32

// NewPlan normalises a declared or reported plan name.
func NewPlan(raw string) Plan {
	plan := plainCharacters.ReplaceAllString(strings.ToUpper(strings.TrimSpace(raw)), "_")
	plan = strings.Trim(plan, "_")
	if len(plan) > maxPlanLength {
		plan = strings.TrimRight(plan[:maxPlanLength], "_")
	}
	return Plan(plan)
}

func (p Plan) String() string { return string(p) }

// IsUnknown reports whether no plan is known.
func (p Plan) IsUnknown() bool { return p == "" }

// BucketID separates the meters of one subscription that has several
// limits (Codex: codex, premium). The empty bucket is the main one.
type BucketID string

const maxBucketLength = 40

// NewBucketID normalises a bucket name reported by a provider.
func NewBucketID(raw string) BucketID {
	bucket := bucketJunk.ReplaceAllString(strings.ToLower(strings.TrimSpace(raw)), "_")
	if len(bucket) > maxBucketLength {
		bucket = bucket[:maxBucketLength]
	}
	return BucketID(bucket)
}

func (b BucketID) String() string { return string(b) }

// IsMain reports whether this is the subscription's main bucket.
func (b BucketID) IsMain() bool { return b == "" }
