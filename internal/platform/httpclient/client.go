// Package httpclient is the only way Ataecina talks to a provider (ADR
// 0006): one fixed HTTPS origin per endpoint, no redirects, no proxy
// unless the owner enables it, explicit timeouts, responses capped at
// 64 KiB after decompression (a larger cap only for the public model
// catalogue, which carries no key, ADR 0015), a plain
// `ataecina/<version>` User-Agent and never a retry.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// MaxBodyBytes caps every response body after decompression.
	MaxBodyBytes   = 64 << 10
	connectTimeout = 5 * time.Second
	requestTimeout = 10 * time.Second
)

// Errors of an exchange; none carries a body or a header value.
var (
	ErrTooLarge   = errors.New("the response is larger than the endpoint accepts")
	ErrRedirected = errors.New("the provider answered with a redirect, which is refused")
	ErrBadOrigin  = errors.New("an endpoint needs an https origin without path, query or credentials")
)

// Options configure the client of one collection.
type Options struct {
	Version             string
	UseEnvironmentProxy bool
	// AllowPlainHTTP is true only in binaries built with the e2e tag,
	// whose fake providers listen on http://127.0.0.1.
	AllowPlainHTTP bool
}

// Client sends requests; it is safe for concurrent use.
type Client struct {
	http      *http.Client
	userAgent string
	plainHTTP bool
}

// New builds a hardened client.
func New(options Options) Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: connectTimeout}).DialContext,
		TLSHandshakeTimeout:   connectTimeout,
		ResponseHeaderTimeout: requestTimeout,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	if options.UseEnvironmentProxy {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return Client{
		http: &http.Client{
			// Each request carries its endpoint's own deadline.
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return ErrRedirected
			},
		},
		userAgent: "ataecina/" + options.Version,
		plainHTTP: options.AllowPlainHTTP,
	}
}

// Endpoint is a client bound to one origin compiled into an adapter.
type Endpoint struct {
	client  Client
	origin  *url.URL
	maxBody int
	timeout time.Duration
}

// For binds the client to an origin such as "https://openrouter.ai".
func (c Client) For(origin string) (Endpoint, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || strings.TrimSuffix(parsed.Path, "/") != "" {
		return Endpoint{}, fmt.Errorf("%w: %q", ErrBadOrigin, origin)
	}
	if parsed.Scheme != "https" && !(c.plainHTTP && parsed.Scheme == "http") {
		return Endpoint{}, fmt.Errorf("%w: %q", ErrBadOrigin, origin)
	}
	return Endpoint{client: c, origin: &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}, maxBody: MaxBodyBytes, timeout: requestTimeout}, nil
}

// ForLoopback binds the client to a local service on 127.0.0.1 (ADR 0013):
// plain HTTP is allowed there because the traffic never leaves the
// machine; the host is fixed and only the port is configurable.
func (c Client) ForLoopback(port int) (Endpoint, error) {
	if port < 1 || port > 65535 {
		return Endpoint{}, fmt.Errorf("%w: loopback port %d", ErrBadOrigin, port)
	}
	host := net.JoinHostPort("127.0.0.1", fmt.Sprint(port))
	return Endpoint{client: c, origin: &url.URL{Scheme: "http", Host: host}, maxBody: MaxBodyBytes, timeout: requestTimeout}, nil
}

// Larger is the endpoint with a larger body cap and more time: only for
// a public catalogue fetched without any key (ADR 0015). It never
// lowers either limit.
func (e Endpoint) Larger(maxBody int, timeout time.Duration) Endpoint {
	e.maxBody, e.timeout = max(e.maxBody, maxBody), max(e.timeout, timeout)
	return e
}

// Host is the endpoint's host, for messages.
func (e Endpoint) Host() string { return e.origin.Host }

// Response is a status, the headers and a capped body.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Get sends one GET to a path of the endpoint's origin.
func (e Endpoint) Get(ctx context.Context, path string, header http.Header) (Response, error) {
	return e.Do(ctx, http.MethodGet, path, header, nil)
}

// Do sends one request to a path (with an optional query) of the origin.
// The path can never change the origin.
func (e Endpoint) Do(ctx context.Context, method, path string, header http.Header, body io.Reader) (Response, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return Response{}, fmt.Errorf("%w: path %q", ErrBadOrigin, path)
	}
	target, err := e.origin.Parse(path)
	if err != nil || target.Host != e.origin.Host || target.Scheme != e.origin.Scheme {
		return Response{}, fmt.Errorf("%w: path %q", ErrBadOrigin, path)
	}
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return Response{}, err
	}
	for name, values := range header {
		request.Header[name] = values
	}
	request.Header.Set("User-Agent", e.client.userAgent)
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}
	response, err := e.client.http.Do(request)
	if err != nil {
		if errors.Is(err, ErrRedirected) {
			return Response{}, ErrRedirected
		}
		return Response{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(e.maxBody)+1))
	if err != nil {
		return Response{}, err
	}
	if len(data) > e.maxBody {
		return Response{}, ErrTooLarge
	}
	return Response{Status: response.StatusCode, Header: response.Header, Body: data}, nil
}
