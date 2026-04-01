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
)

var sharedHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

func cloneTransportConfig() *http.Transport {
	base, ok := sharedHTTPClient.Transport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	return &http.Transport{
		MaxIdleConns:        base.MaxIdleConns,
		MaxConnsPerHost:     base.MaxConnsPerHost,
		MaxIdleConnsPerHost: base.MaxIdleConnsPerHost,
		IdleConnTimeout:     base.IdleConnTimeout,
		TLSHandshakeTimeout: base.TLSHandshakeTimeout,
	}
}

const maxResponseSize = 2 * 1024 * 1024 // 2MB

// FetchBytes fetches the given URL and returns the raw response body as bytes.
func FetchBytes(ctx context.Context, url string, fetchTimeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s: %w", url, err)
	}
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed for %s: %w", url, err)
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
		return nil, fmt.Errorf("unexpected status code: %d for url %s", resp.StatusCode, url)
	}
	limitReader := io.LimitReader(resp.Body, maxResponseSize)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", url, err)
	}
	return data, nil
}

func FetchJSON[T any](ctx context.Context, url string, fetchTimeout time.Duration) (T, error) {
	return getJSONWithClient[T](ctx, url, fetchTimeout, sharedHTTPClient, "")
}

// FetchJSONPinned fetches JSON from url while pinning the connection to dialAddr (host:port).
// hostHeader is used as the HTTP Host header; serverName is used for TLS SNI.
func FetchJSONPinned[T any](ctx context.Context, url string, fetchTimeout time.Duration, dialAddr, hostHeader, serverName string) (T, error) {
	transport := cloneTransportConfig()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, network, dialAddr)
	}
	transport.TLSClientConfig = &tls.Config{ServerName: serverName}

	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()
	return getJSONWithClient[T](ctx, url, fetchTimeout, client, hostHeader)
}

func getJSONWithClient[T any](ctx context.Context, url string, fetchTimeout time.Duration, client *http.Client, hostHeader string) (T, error) {
	var zero T
	// per-request timeout
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, fmt.Errorf("failed to create HTTP request for %s: %w", url, err)
	}
	if hostHeader != "" {
		req.Host = hostHeader
	}
	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("HTTP request failed for %s: %w: %w", url, err, ErrHTTPFetch)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body for %s: %v", url, err)
		}
	}()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return zero, ErrNotFound
	case http.StatusOK:
		// proceed
	default:
		return zero, fmt.Errorf("unexpected status code: %d for url %s: %w", resp.StatusCode, url, ErrHTTPFetch)
	}
	limitReader := io.LimitReader(resp.Body, maxResponseSize)
	err = json.NewDecoder(limitReader).Decode(&zero)
	if err != nil {
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
