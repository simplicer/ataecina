// Package keycheck reads the providers that publish no allowance a key can
// read without spending a request (SPECS §3.7): Groq, NVIDIA and Google AI
// Studio. It sends one free, read-only request that only a valid key is
// answered, and reports the subscription UNSUPPORTED with a hint that the
// key works, or the key rejected. What the subscription offers comes from
// the model catalogue (offerings), not from this request: nothing of the
// answer is read but the presence of its list.
package keycheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/providerhttp"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Provider is one provider's fixed check.
type Provider struct {
	// Name is how the hint names the provider.
	Name string
	// Origin is the only host contacted.
	Origin string
	// Path is a free, read-only request that only a valid key is answered.
	Path string
	// KeyHeader carries the bare key; empty for "Authorization: Bearer".
	KeyHeader string
	// BadKey is a status that also means a rejected key (Google: 400).
	BadKey int
	// List is the member of a good answer that holds an array.
	List string
}

var (
	// Groq lists its models only to a valid key (401 otherwise).
	Groq = Provider{Name: "Groq", Origin: "https://api.groq.com", Path: "/openai/v1/models", List: "data"}
	// NVIDIA lists the key owner's private NVCF functions (usually none);
	// the public model list answers any key.
	NVIDIA = Provider{
		Name: "NVIDIA", Origin: "https://api.nvcf.nvidia.com",
		Path: "/v2/nvcf/functions?visibility=private", List: "functions",
	}
	// Gemini lists one model; Google answers an unknown key with 400.
	Gemini = Provider{
		Name: "Google AI Studio", Origin: "https://generativelanguage.googleapis.com",
		Path: "/v1beta/models?pageSize=1", KeyHeader: "x-goog-api-key", BadKey: http.StatusBadRequest, List: "models",
	}
)

// Source checks one key of one provider.
type Source struct {
	provider Provider
	endpoint httpclient.Endpoint
	keys     domain.KeyRing
	key      domain.KeyName
	now      func() time.Time
}

// NewSource checks the named key through endpoint.
func (p Provider) NewSource(endpoint httpclient.Endpoint, keys domain.KeyRing, key domain.KeyName, now func() time.Time) Source {
	return Source{provider: p, endpoint: endpoint, keys: keys, key: key, now: now}
}

// Read sends the check once.
func (s Source) Read(ctx context.Context, subscription domain.Subscription) (domain.Reading, error) {
	key, err := s.keys.KeyFor(ctx, s.key)
	if err != nil {
		return domain.Reading{}, err
	}
	defer key.Wipe()
	header := http.Header{}
	if s.provider.KeyHeader == "" {
		header.Set("Authorization", "Bearer "+string(key.Reveal()))
	} else {
		header.Set(s.provider.KeyHeader, string(key.Reveal()))
	}
	now := s.now()
	response, err := providerhttp.Send(ctx, s.endpoint, http.MethodGet, s.provider.Path, header, nil)
	if err != nil {
		return domain.Reading{}, err
	}
	host := s.endpoint.Host()
	if s.provider.BadKey != 0 && response.Status == s.provider.BadKey {
		return domain.Reading{}, providerhttp.KeyRejected(host, response.Status)
	}
	if failure := providerhttp.Classify(host, response, now); failure != nil {
		return domain.Reading{}, failure
	}
	if err := DecodeList(response.Body, s.provider.List); err != nil {
		return domain.Reading{}, providerhttp.Unexpected(host, err)
	}
	return domain.NewReading(domain.ReadingFacts{
		Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
		Plan: subscription.Plan(), Fidelity: domain.FidelityReported, Outcome: domain.OutcomeUnsupported, ObservedAt: now,
		Hint: s.provider.Name + " publishes no allowance a key can read without spending a request; the key works",
	})
}

// DecodeList checks a good answer: a JSON object whose member list holds
// an array (possibly empty).
func DecodeList(body []byte, list string) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return err
	}
	raw, ok := document[list]
	if !ok {
		return fmt.Errorf("%s is missing", list)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Errorf("%s is not a list: %w", list, err)
	}
	if items == nil {
		return errors.New(list + " is null")
	}
	return nil
}
