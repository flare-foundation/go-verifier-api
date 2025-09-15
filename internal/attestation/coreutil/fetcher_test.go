package coreutil

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	want := 42
	op := func() (int, error) {
		return want, nil
	}
	got, err := Retry(3, time.Millisecond, op, nil)
	require.NoError(t, err, "expected no error")
	require.Equal(t, want, got, "expected %d, got %d", want, got)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary failure")
		}
		return "ok", nil
	}
	got, err := Retry(5, time.Millisecond, op, nil)
	require.NoError(t, err, "expected success")
	require.Equal(t, "ok", got, "expected ok, got %s", got)
	require.Equal(t, 3, attempts, "expected 3 attempts, got %d", attempts)
}

func TestRetry_ExhaustRetries(t *testing.T) {
	op := func() (int, error) {
		return 0, errors.New("always fails")
	}
	_, err := Retry(3, time.Millisecond, op, nil)
	require.Error(t, err, "expected error, got nil")
}

func TestRetry_BreakOn(t *testing.T) {
	specialErr := errors.New("stop now")
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts == 2 {
			return "bad", specialErr
		}
		return "", errors.New("regular failure")
	}
	got, err := Retry(5, time.Millisecond, op, func(e error) bool {
		return errors.Is(e, specialErr)
	})
	require.ErrorIs(t, err, specialErr, "expected specialErr, got %v", err)
	require.Equal(t, "bad", got, "expected bad, got %s", got)
	require.Equal(t, 2, attempts, "expected 2 attempts, got %d", attempts)
}

func TestRetry_ReturnsLastResult(t *testing.T) {
	op := func() (int, error) {
		return 99, errors.New("fail but keep result")
	}
	got, err := Retry(2, time.Millisecond, op, nil)
	require.Error(t, err)
	require.Equal(t, 99, got)
}
