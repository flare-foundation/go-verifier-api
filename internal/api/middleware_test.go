package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

type emptyInput struct{}
type emptyOutput struct {
	Body string `json:"body"`
}

func setupTestAPI(t *testing.T, apiKeys []string) (huma.API, *chi.Mux) {
	t.Helper()
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("test", "1.0"))
	api.UseMiddleware(APIKeyAuthMiddleware(api, apiKeys))
	huma.Get(api, "/api/health", func(ctx context.Context, input *emptyInput) (*emptyOutput, error) {
		return &emptyOutput{Body: "ok"}, nil
	})
	huma.Get(api, "/api/protected", func(ctx context.Context, input *emptyInput) (*emptyOutput, error) {
		return &emptyOutput{Body: "secret"}, nil
	})
	return api, router
}

func TestAPIKeyAuthMiddleware(t *testing.T) {
	_, router := setupTestAPI(t, []string{"valid-key-1", "valid-key-2"})

	t.Run("valid key grants access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("X-API-KEY", "valid-key-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("second valid key also works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("X-API-KEY", "valid-key-2")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("wrong key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("X-API-KEY", "wrong-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("empty key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("X-API-KEY", "")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("health endpoint bypasses auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		// No X-API-KEY header.
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("health endpoint with wrong key still works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("X-API-KEY", "wrong-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("health-like paths still require auth", func(t *testing.T) {
		for _, path := range []string{"/api/healthz", "/api/health/extra", "/api/health/"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// These paths don't match /api/health exactly, so auth should apply.
			// Without a valid key, expect 401 or 404 (not 200).
			require.NotEqual(t, http.StatusOK, w.Code, "path %q should not bypass auth", path)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	handler := newSecurityHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("X-Frame-Options is DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	})

	t.Run("X-Content-Type-Options is nosniff", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	})

	t.Run("both headers present on every response", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			req := httptest.NewRequest(method, "/any-path", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			require.Equal(t, "DENY", w.Header().Get("X-Frame-Options"), "method: %s", method)
			require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), "method: %s", method)
		}
	})
}
