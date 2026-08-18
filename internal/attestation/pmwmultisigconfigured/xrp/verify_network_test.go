package xrpverifier

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func serverInfoServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

func verifierFor(url string, source config.SourceName) *XRPVerifier {
	cfg := &config.PMWMultisigAccountConfig{SourceRPCURL: url}
	cfg.SourceIDPair.SourceID = source
	return NewXRPVerifier(cfg)
}

// TestVerifyNetwork covers the startup pin: only a confirmed wrong chain fails
// boot; an unreachable node or one that reports no network_id does not block boot
// (the request gate enforces fail-closed instead).
func TestVerifyNetwork(t *testing.T) {
	t.Run("matching network passes", func(t *testing.T) {
		s := serverInfoServer(t, `{"result":{"status":"success","info":{"network_id":1}}}`)
		defer s.Close()
		require.NoError(t, verifierFor(s.URL, config.SourceTestXRP).VerifyNetwork(context.Background()))
	})

	t.Run("wrong network fails boot", func(t *testing.T) {
		// Mainnet node (0) configured for a testXRP deployment (expects 1).
		s := serverInfoServer(t, `{"result":{"status":"success","info":{"network_id":0}}}`)
		defer s.Close()
		require.ErrorIs(t, verifierFor(s.URL, config.SourceTestXRP).VerifyNetwork(context.Background()), ErrNetworkMismatch)
	})

	t.Run("unreachable node does not block boot", func(t *testing.T) {
		s := serverInfoServer(t, `{}`)
		s.Close() // now unreachable
		require.NoError(t, verifierFor(s.URL, config.SourceXRP).VerifyNetwork(context.Background()))
	})

	t.Run("absent network id does not block boot", func(t *testing.T) {
		s := serverInfoServer(t, `{"result":{"status":"success","info":{}}}`)
		defer s.Close()
		require.NoError(t, verifierFor(s.URL, config.SourceXRP).VerifyNetwork(context.Background()))
	})
}

// TestEnsureNetworkVerified covers the request gate: it fails closed until the
// network is confirmed, then caches the result.
func TestEnsureNetworkVerified(t *testing.T) {
	t.Run("matching network verifies and caches", func(t *testing.T) {
		s := serverInfoServer(t, `{"result":{"status":"success","info":{"network_id":1}}}`)
		v := verifierFor(s.URL, config.SourceTestXRP)
		require.NoError(t, v.ensureNetworkVerified(context.Background()))
		s.Close() // now unreachable, but the confirmed result is cached
		require.NoError(t, v.ensureNetworkVerified(context.Background()))
	})

	t.Run("wrong network fails closed", func(t *testing.T) {
		s := serverInfoServer(t, `{"result":{"status":"success","info":{"network_id":0}}}`)
		defer s.Close()
		require.ErrorIs(t, verifierFor(s.URL, config.SourceTestXRP).ensureNetworkVerified(context.Background()), ErrNetworkMismatch)
	})

	t.Run("unreachable node fails closed", func(t *testing.T) {
		s := serverInfoServer(t, `{}`)
		s.Close() // now unreachable
		require.Error(t, verifierFor(s.URL, config.SourceXRP).ensureNetworkVerified(context.Background()))
	})

	t.Run("re-verifies after the TTL and catches a repointed endpoint", func(t *testing.T) {
		// network_id is served in Clio's string form and can change between calls.
		netID := "1"
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"result":{"status":"success","info":{"network_id":%q}}}`, netID)
		}))
		defer s.Close()

		v := verifierFor(s.URL, config.SourceTestXRP)
		clock := time.Unix(1000, 0)
		v.now = func() time.Time { return clock }

		require.NoError(t, v.ensureNetworkVerified(context.Background())) // confirmed testnet

		// Endpoint repointed to Mainnet; within the TTL the cached result still serves.
		netID = "0"
		require.NoError(t, v.ensureNetworkVerified(context.Background()))

		// Past the TTL it re-probes and catches the wrong network.
		clock = clock.Add(networkVerifyTTL + time.Second)
		require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), ErrNetworkMismatch)
	})
}
