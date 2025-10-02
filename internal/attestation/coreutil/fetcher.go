package coreutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

func GetJSON[T any](ctx context.Context, url string, fetchTimeout time.Duration) (T, error) {
	var zero T
	httpClient := &http.Client{
		Timeout: fetchTimeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, fmt.Errorf("failed to create HTTP request for %s: %w", url, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("HTTP request failed for: %w", err)
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
		return zero, fmt.Errorf("unexpected status code: %d for url %s", resp.StatusCode, url)
	}
	err = json.NewDecoder(resp.Body).Decode(&zero)
	if err != nil {
		return zero, fmt.Errorf("decoding JSON from %s failed for type %s: %w", url, reflect.TypeOf(zero), err)
	}
	return zero, nil
}

func Retry[T any](
	maxAttempts int,
	delay time.Duration,
	operation func() (T, error),
	breakOn func(error) bool,
) (T, error) {
	var lastErr error
	var lastResult T
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
			time.Sleep(delay)
		}
	}
	return lastResult, lastErr
}
