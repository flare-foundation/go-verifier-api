package coreutil

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		want := 42
		op := func() (int, error) { return want, nil }
		got, err := Retry(3, time.Millisecond, op, nil)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		op := func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("temporary failure")
			}
			return "ok", nil
		}
		got, err := Retry(5, time.Millisecond, op, nil)
		require.NoError(t, err)
		require.Equal(t, "ok", got)
		require.Equal(t, 3, attempts)
	})
	t.Run("exhaust retries", func(t *testing.T) {
		op := func() (int, error) { return 0, errors.New("always fails") }
		val, err := Retry(3, time.Millisecond, op, nil)
		require.ErrorContains(t, err, "always fails")
		require.Equal(t, 0, val)
	})
	t.Run("break on special error", func(t *testing.T) {
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
		require.ErrorIs(t, err, specialErr)
		require.Equal(t, "bad", got)
		require.Equal(t, 2, attempts)
	})
	t.Run("returns last result on failure", func(t *testing.T) {
		op := func() (int, error) { return 99, errors.New("fail but keep result") }
		got, err := Retry(2, time.Millisecond, op, nil)
		require.ErrorContains(t, err, "fail but keep result")
		require.Equal(t, 99, got)
	})
	t.Run("zero maxAttempts returns zero value", func(t *testing.T) {
		op := func() (int, error) { return 123, errors.New("should not run") }
		got, err := Retry(0, time.Millisecond, op, nil)
		require.NoError(t, err)
		require.Equal(t, 0, got)
	})
	t.Run("breakOn true on first attempt stops retrying", func(t *testing.T) {
		attempts := 0
		specialErr := errors.New("break immediately")
		op := func() (string, error) {
			attempts++
			return "", specialErr
		}
		got, err := Retry(5, time.Millisecond, op, func(e error) bool {
			return true
		})
		require.ErrorIs(t, err, specialErr)
		require.Equal(t, "", got)
		require.Equal(t, 1, attempts, "should break after first attempt")
	})
}
