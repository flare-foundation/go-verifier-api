package fetcher

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetchBytes(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		expected := []byte("hello world")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(expected)
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 5*time.Second)
		require.NoError(t, err)
		require.Equal(t, expected, data)
	})
	t.Run("404 returns ErrNotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 5*time.Second)
		require.ErrorIs(t, err, ErrNotFound)
		require.Nil(t, data)
	})
	t.Run("unexpected status code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 5*time.Second)
		require.ErrorContains(t, err, "unexpected status code: 500")
		require.Nil(t, data)
	})
	t.Run("response truncated at maxResponseSize", func(t *testing.T) {
		// Serve more than maxResponseSize bytes
		bigBody := strings.Repeat("x", maxResponseSize+100)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(bigBody))
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 5*time.Second)
		require.NoError(t, err)
		require.Len(t, data, maxResponseSize)
	})
	t.Run("redirect is rejected", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("redirected"))
		}))
		defer target.Close()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 5*time.Second)
		require.ErrorIs(t, err, ErrRedirect)
		require.Nil(t, data)
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		data, err := FetchBytes(ctx, server.URL, 100*time.Millisecond)
		require.Error(t, err)
		require.Nil(t, data)
	})
}

func TestRetry(t *testing.T) {
	ctx := context.Background()
	t.Run("success on first attempt", func(t *testing.T) {
		want := 42
		op := func() (int, error) { return want, nil }
		got, err := Retry(ctx, 3, time.Millisecond, op, nil)
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
		got, err := Retry(ctx, 5, time.Millisecond, op, nil)
		require.NoError(t, err)
		require.Equal(t, "ok", got)
		require.Equal(t, 3, attempts)
	})
	t.Run("exhaust retries", func(t *testing.T) {
		op := func() (int, error) { return 0, errors.New("always fails") }
		val, err := Retry(ctx, 3, time.Millisecond, op, nil)
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
		got, err := Retry(ctx, 5, time.Millisecond, op, func(e error) bool {
			return errors.Is(e, specialErr)
		})
		require.ErrorIs(t, err, specialErr)
		require.Equal(t, "bad", got)
		require.Equal(t, 2, attempts)
	})
	t.Run("returns last result on failure", func(t *testing.T) {
		op := func() (int, error) { return 99, errors.New("fail but keep result") }
		got, err := Retry(ctx, 2, time.Millisecond, op, nil)
		require.ErrorContains(t, err, "fail but keep result")
		require.Equal(t, 99, got)
	})
	t.Run("zero maxAttempts returns error", func(t *testing.T) {
		op := func() (int, error) { return 123, errors.New("should not run") }
		_, err := Retry(ctx, 0, time.Millisecond, op, nil)
		require.Error(t, err)
	})
	t.Run("breakOn true on first attempt stops retrying", func(t *testing.T) {
		attempts := 0
		specialErr := errors.New("break immediately")
		op := func() (string, error) {
			attempts++
			return "", specialErr
		}
		got, err := Retry(ctx, 5, time.Millisecond, op, func(e error) bool {
			return true
		})
		require.ErrorIs(t, err, specialErr)
		require.Equal(t, "", got)
		require.Equal(t, 1, attempts, "should break after first attempt")
	})
	t.Run("context cancellation during retry delay", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		attempts := 0
		op := func() (int, error) {
			attempts++
			if attempts == 1 {
				cancel() // cancel context after first failed attempt
			}
			return 0, errors.New("fail")
		}
		_, err := Retry(cancelCtx, 5, time.Second, op, nil)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 1, attempts, "should not retry after context cancellation")
	})
}

func TestFetchJSONPinnedUsesHostHeader(t *testing.T) {
	wantHost := "example.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != wantHost {
			http.Error(w, "bad host", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	dialAddr := server.Listener.Addr().String()
	_, _, err := net.SplitHostPort(dialAddr)
	require.NoError(t, err)

	url := server.URL + "/"
	got, err := FetchJSONPinned[struct {
		OK bool `json:"ok"`
	}](context.Background(), url, 2*time.Second, dialAddr, wantHost, "")
	require.NoError(t, err)
	require.True(t, got.OK)
}
