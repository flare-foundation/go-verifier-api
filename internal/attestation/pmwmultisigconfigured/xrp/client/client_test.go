package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/types"
	"github.com/stretchr/testify/require"
)

func TestFetchAccountInfo(t *testing.T) {
	expected := types.AccountInfoResponse{
		Result: types.AccountInfoResult{
			Status: "success",
			AccountData: types.AccountData{
				Account: "rEXAMPLE",
			},
		},
	}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		wantErr     string
		wantAccount string
		ctx         func() context.Context
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			},
			wantAccount: "rEXAMPLE",
		},
		{
			name: "error response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			},
			wantErr: "cannot get account info: all attempts failed. Last error: request responded with code 500, reason: Internal Server Error",
		},
		{
			name: "bad JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"result": { "status": "success", "account_data": `)) // invalid JSON
				require.NoError(t, err)
			},
			wantErr: "cannot get account info: all attempts failed. Last error: decoding response:",
		},
		{
			name: "non-success status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := types.AccountInfoResponse{
					Result: types.AccountInfoResult{Status: "error"},
				}
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			},
			wantErr: "XRP RPC returned non-success status",
		},
		{
			name: "context timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond)
			},
			wantErr: "context deadline exceeded",
			ctx: func() context.Context {
				c, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			resp, err := client.FetchAccountInfo(ctx, "rEXAMPLE")

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, resp)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantAccount, resp.Result.AccountData.Account)
		})
	}
}

// TestFetchAccountInfoStatusClassification: a transient node status is retryable
// (ErrRPCTransient) and must NOT be classifiable as the terminal ErrRPCNonSuccess,
// while a deterministic negative (actNotFound) stays terminal.
func TestFetchAccountInfoStatusClassification(t *testing.T) {
	statusServer := func(status string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(types.AccountInfoResponse{
				Result: types.AccountInfoResult{Status: status},
			})
		}))
	}
	t.Run("transient status is retryable", func(t *testing.T) {
		srv := statusServer("tooBusy")
		defer srv.Close()
		_, err := NewClient(srv.URL).FetchAccountInfo(context.Background(), "rEXAMPLE")
		require.ErrorIs(t, err, ErrRPCTransient)
		require.NotErrorIs(t, err, ErrRPCNonSuccess, "a transient node state must not be a terminal rejection")
	})
	t.Run("actNotFound is terminal", func(t *testing.T) {
		srv := statusServer("actNotFound")
		defer srv.Close()
		_, err := NewClient(srv.URL).FetchAccountInfo(context.Background(), "rEXAMPLE")
		require.ErrorIs(t, err, ErrRPCNonSuccess)
		require.NotErrorIs(t, err, ErrRPCTransient)
	})
}

func TestNetworkID(t *testing.T) {
	t.Run("reports the network id", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"result":{"status":"success","info":{"network_id":1}}}`))
		}))
		defer server.Close()

		id, present, err := NewClient(server.URL).NetworkID(context.Background())
		require.NoError(t, err)
		require.True(t, present)
		require.Equal(t, uint32(1), id)
	})

	t.Run("clio string network id is accepted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"result":{"status":"success","info":{"network_id":"1"}}}`)) // Clio reports it as a string
		}))
		defer server.Close()

		id, present, err := NewClient(server.URL).NetworkID(context.Background())
		require.NoError(t, err)
		require.True(t, present)
		require.Equal(t, uint32(1), id)
	})

	t.Run("absent network id is not present", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"result":{"status":"success","info":{}}}`))
		}))
		defer server.Close()

		_, present, err := NewClient(server.URL).NetworkID(context.Background())
		require.NoError(t, err)
		require.False(t, present)
	})

	t.Run("non-success status is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"result":{"status":"error"}}`))
		}))
		defer server.Close()

		_, _, err := NewClient(server.URL).NetworkID(context.Background())
		require.ErrorIs(t, err, ErrFetchServerInfo)
	})
}
