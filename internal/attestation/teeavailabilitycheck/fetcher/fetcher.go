package fetcher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

var (
	ErrNotFound  = errors.New("resource not found (404)")
	ErrHTTPFetch = errors.New("HTTP fetch failed")
	ErrRedirect  = errors.New("redirects are not allowed")
)

// HTTPStatusError is returned when an HTTP response carries a non-2xx status
// code. It exposes StatusCode so callers (and shared error classifiers) can
// distinguish deterministic client errors (4xx) from transient server errors
// (5xx) without string parsing.
type HTTPStatusError struct {
	URL  string
	Code int
}

// StatusCode implements the statusCoder interface used by shared error
// classifiers.
func (e *HTTPStatusError) StatusCode() int { return e.Code }

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d for url %s", e.Code, e.URL)
}

func (e *HTTPStatusError) Unwrap() error { return ErrHTTPFetch }

// noRedirects rejects any HTTP redirect. Following redirects would bypass the
// SSRF controls applied to the original URL.
var noRedirects = func(_ *http.Request, _ []*http.Request) error {
	return ErrRedirect
}

var sharedTransportTemplate = &http.Transport{
	MaxIdleConns:           100,
	MaxConnsPerHost:        100,
	MaxIdleConnsPerHost:    100,
	IdleConnTimeout:        90 * time.Second,
	TLSHandshakeTimeout:    10 * time.Second,
	MaxResponseHeaderBytes: 1 << 20, // 1 MB
	ResponseHeaderTimeout:  5 * time.Second,
}

func cloneTransportConfig() *http.Transport {
	return &http.Transport{
		MaxIdleConns:           sharedTransportTemplate.MaxIdleConns,
		MaxConnsPerHost:        sharedTransportTemplate.MaxConnsPerHost,
		MaxIdleConnsPerHost:    sharedTransportTemplate.MaxIdleConnsPerHost,
		IdleConnTimeout:        sharedTransportTemplate.IdleConnTimeout,
		TLSHandshakeTimeout:    sharedTransportTemplate.TLSHandshakeTimeout,
		MaxResponseHeaderBytes: sharedTransportTemplate.MaxResponseHeaderBytes,
		ResponseHeaderTimeout:  sharedTransportTemplate.ResponseHeaderTimeout,
	}
}

const maxResponseSize = 2 * 1024 * 1024 // 2MB

// FetchBytesPinned fetches raw bytes from url while pinning the connection to
// dialAddr (host:port). hostHeader is used as the HTTP Host header; serverName
// is used for TLS SNI. The caller is expected to have already resolved url to
// a safe IP via ResolveExternalURL — this function only enforces the pinned
// dial, response-size cap, and no-redirect policy.
func FetchBytesPinned(ctx context.Context, url string, fetchTimeout time.Duration, dialAddr, hostHeader, serverName string) ([]byte, error) {
	transport := cloneTransportConfig()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, network, dialAddr)
	}
	transport.TLSClientConfig = &tls.Config{ServerName: serverName}

	client := &http.Client{Transport: transport, CheckRedirect: noRedirects}
	defer transport.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s: %w", url, err)
	}
	if hostHeader != "" {
		req.Host = hostHeader
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed for %s: %w: %w", url, err, ErrHTTPFetch)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body for %s: %v", url, err)
		}
	}()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusOK:
		// proceed
	default:
		return nil, &HTTPStatusError{URL: url, Code: resp.StatusCode}
	}
	limitReader := io.LimitReader(resp.Body, maxResponseSize)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", url, err)
	}
	return data, nil
}

// FetchJSONPinned fetches JSON from url with the same pinning + size + redirect
// guarantees as FetchBytesPinned, then unmarshals into T.
func FetchJSONPinned[T any](ctx context.Context, url string, fetchTimeout time.Duration, dialAddr, hostHeader, serverName string) (T, error) {
	var zero T
	body, err := FetchBytesPinned(ctx, url, fetchTimeout, dialAddr, hostHeader, serverName)
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(body, &zero); err != nil {
		return zero, fmt.Errorf("decoding JSON from %s failed for type %s: %w", url, reflect.TypeOf(zero), err)
	}
	return zero, nil
}

func Retry[T any](
	ctx context.Context,
	maxAttempts int,
	delay time.Duration,
	operation func() (T, error),
	breakOn func(error) bool,
) (T, error) {
	var lastErr error
	var lastResult T
	if maxAttempts <= 0 {
		return lastResult, errors.New("maxAttempts must be at least 1")
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}
		if breakOn != nil && breakOn(err) {
			return result, err
		}
		lastErr = err
		lastResult = result
		logger.Warnf("Attempt %d/%d failed: %v", attempt, maxAttempts, err)
		if attempt < maxAttempts {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return lastResult, ctx.Err()
			}
		}
	}
	return lastResult, lastErr
}
