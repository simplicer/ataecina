// Package providerhttp asks a provider API with a sealed key and
// classifies every failure into the domain's error classes (SPECS §4.3).
// No message ever carries a key, a header value or a response body.
package providerhttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// Fetch sends one GET with "Authorization: Bearer <key>" and returns the
// body of a 2xx answer; anything else is a *domain.SourceFailure.
func Fetch(ctx context.Context, endpoint httpclient.Endpoint, path string, key secret.Key, now time.Time) ([]byte, error) {
	return ask(ctx, endpoint, http.MethodGet, path, "Bearer "+string(key.Reveal()), nil, now)
}

// Post sends one POST of a fixed JSON document, like Fetch.
func Post(ctx context.Context, endpoint httpclient.Endpoint, path string, key secret.Key, document []byte, now time.Time) ([]byte, error) {
	return ask(ctx, endpoint, http.MethodPost, path, "Bearer "+string(key.Reveal()), document, now)
}

// PostBasic is Post for providers that take the key as the Basic user
// name with an empty password ("Authorization: Basic base64(key:)").
func PostBasic(ctx context.Context, endpoint httpclient.Endpoint, path string, key secret.Key, document []byte, now time.Time) ([]byte, error) {
	return ask(ctx, endpoint, http.MethodPost, path, "Basic "+basicCredentials(key), document, now)
}

// basicCredentials encodes "key:" from a private copy, wiped once encoded,
// so the key's own bytes are never appended to.
func basicCredentials(key secret.Key) string {
	credentials := make([]byte, 0, key.Len()+1)
	credentials = append(append(credentials, key.Reveal()...), ':')
	defer clear(credentials)
	return base64.StdEncoding.EncodeToString(credentials)
}

func ask(ctx context.Context, endpoint httpclient.Endpoint, method, path, authorization string, document []byte, now time.Time) ([]byte, error) {
	header := http.Header{}
	header.Set("Authorization", authorization)
	response, err := Send(ctx, endpoint, method, path, header, document)
	if err != nil {
		return nil, err
	}
	if failure := Classify(endpoint.Host(), response, now); failure != nil {
		return nil, failure
	}
	return response.Body, nil
}

// Send sends one request with header (which carries the key, as the
// provider takes it) and a JSON document when not nil, and returns the
// answer whatever its status: only failing to get one is a
// *domain.SourceFailure. Callers classify the status, with Classify or
// with a rule of their provider's.
func Send(ctx context.Context, endpoint httpclient.Endpoint, method, path string, header http.Header, document []byte) (httpclient.Response, error) {
	var body io.Reader
	if document != nil {
		header.Set("Content-Type", "application/json")
		body = bytes.NewReader(document)
	}
	response, err := endpoint.Do(ctx, method, path, header, body)
	switch {
	case errors.Is(err, httpclient.ErrTooLarge):
		return httpclient.Response{}, domain.Failure(domain.ErrorContractChanged, endpoint.Host()+" answered more than 64 KiB", nil)
	case errors.Is(err, httpclient.ErrRedirected):
		return httpclient.Response{}, domain.Failure(domain.ErrorContractChanged, endpoint.Host()+" answered with a redirect, which is refused", nil)
	case err != nil:
		return httpclient.Response{}, domain.Failure(domain.ErrorUnavailable, endpoint.Host()+" did not answer", err)
	}
	return response, nil
}

// Unexpected is the failure for an answer whose shape is not the known one.
func Unexpected(host string, err error) *domain.SourceFailure {
	return domain.Failure(domain.ErrorContractChanged, host+" answered in an unexpected shape", err)
}

// Classify maps a status to a failure, or nil for 2xx.
func Classify(host string, response httpclient.Response, now time.Time) *domain.SourceFailure {
	status := response.Status
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return KeyRejected(host, status)
	case status == http.StatusTooManyRequests:
		if retryAt, ok := RetryAfter(response.Header.Get("Retry-After"), now); ok {
			return domain.RateLimited(retryAt, host+" asked to wait")
		}
		return domain.Failure(domain.ErrorRateLimited, host+" asked to wait", nil)
	case status >= 500:
		return domain.Failure(domain.ErrorUnavailable, fmt.Sprintf("%s is failing (HTTP %d)", host, status), nil)
	}
	return domain.Failure(domain.ErrorContractChanged, fmt.Sprintf("%s answered HTTP %d", host, status), nil)
}

// KeyRejected is the failure for a key the provider refused with status.
func KeyRejected(host string, status int) *domain.SourceFailure {
	return domain.Failure(domain.ErrorKeyRejected, fmt.Sprintf("%s rejected the key (HTTP %d): set it again with `ataecina keys set`", host, status), nil)
}

// RetryAfter reads a Retry-After header: seconds or an HTTP date.
func RetryAfter(value string, now time.Time) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return now.Add(time.Duration(seconds) * time.Second), true
	}
	if at, err := http.ParseTime(value); err == nil {
		return at, true
	}
	return time.Time{}, false
}

// USD is the currency of every provider balance Ataecina reads today.
func USD() domain.Currency {
	usd, _ := domain.NewCurrency("USD")
	return usd
}

// Dollars converts a provider figure to money.
func Dollars(value float64) (domain.Money, error) {
	amount, err := domain.AmountFromFloat(value)
	if err != nil {
		return domain.Money{}, err
	}
	return domain.NewMoney(amount, USD()), nil
}
