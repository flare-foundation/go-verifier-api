package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/type"
	"github.com/stretchr/testify/require"
)

func TestGetAccountInfo(t *testing.T) {
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
				json.NewEncoder(w).Encode(expected)
			},
			wantAccount: "rEXAMPLE",
		},
		{
			name: "error response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			},
			wantErr: "cannot get account info: max retries reached: request responded with code 500, reason: Internal Server Error",
		},
		{
			name: "bad JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"result": { "status": "success", "account_data": `)) // invalid JSON
			},
			wantErr: "cannot get account info: max retries reached: decoding response:",
		},
		{
			name: "non-success status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := types.AccountInfoResponse{
					Result: types.AccountInfoResult{Status: "error"},
				}
				json.NewEncoder(w).Encode(resp)
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

			resp, err := client.GetAccountInfo(ctx, "rEXAMPLE")

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
