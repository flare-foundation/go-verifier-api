package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestServer returns an httptest server that responds to every request with
// body, plus a Client pointed at it.
func newTestServer(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

// TestGetTxOutSuccess confirms a populated result unmarshals into the UTXO view.
func TestGetTxOutSuccess(t *testing.T) {
	c := newTestServer(t, `{"result":{"confirmations":6,"value":0.0001,"scriptPubKey":{"hex":"0014abcd","type":"witness_v0_keyhash"}},"error":null,"id":"go-verifier-api"}`)
	out, err := c.GetTxOut(context.Background(), "aa", 0, false)
	require.NoError(t, err)
	require.NotNil(t, out)
	sat, err := out.ValueSat()
	require.NoError(t, err)
	require.Equal(t, int64(10_000), sat)
}

// TestGetTxOutNullResult confirms an unspent-not-found / spent output (null
// result) returns a nil view with no error.
func TestGetTxOutNullResult(t *testing.T) {
	c := newTestServer(t, `{"result":null,"error":null,"id":"go-verifier-api"}`)
	out, err := c.GetTxOut(context.Background(), "aa", 0, false)
	require.NoError(t, err)
	require.Nil(t, out)
}

// TestGetTxOutRPCErrorsAreTransient confirms EVERY gettxout RPC error maps to the
// retryable ErrGetTxOut (503) — data codes (-3/-8/-22), protocol/method codes
// (-32601), and warmup (-28) alike. gettxout always gets a well-formed txid/vout
// and returns null (not an error) for a missing output, so any RPC error is a
// node/client fault, never bad caller data; a false 4xx would turn an
// abstain-worthy outage into a hard "invalid request".
func TestGetTxOutRPCErrorsAreTransient(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{"type error", -3},
		{"invalid parameter", -8},
		{"deserialization", -22},
		{"warmup", -28},
		{"method not found", -32601},
		{"parse error", -32700},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestServer(t, fmt.Sprintf(`{"result":null,"error":{"code":%d,"message":"x"},"id":"go-verifier-api"}`, tc.code))
			_, err := c.GetTxOut(context.Background(), "aa", 0, false)
			require.ErrorIs(t, err, ErrGetTxOut)
		})
	}
}

// TestGetTxOutInFlightCapFailsFast confirms that once the in-flight RPC cap is
// full, a further call fails fast with ErrGetTxOut instead of piling up.
func TestGetTxOutInFlightCapFailsFast(t *testing.T) {
	c := NewClient("http://127.0.0.1:1/")
	for range cap(c.sem) {
		c.sem <- struct{}{} // saturate the semaphore
	}
	_, err := c.GetTxOut(context.Background(), "aa", 0, false)
	require.ErrorIs(t, err, ErrGetTxOut)
	require.ErrorIs(t, err, errTooManyConcurrent)
}

// TestGetTxOutDoesNotLeakCredentials confirms a transport failure on a URL that
// carries HTTP basic-auth credentials in its userinfo (the documented bitcoind
// config, e.g. http://user:pass@host) does not surface the password in the
// error — which is wrapped and logged verbatim by the handler. net/http's
// url.Error redacts the password to "***"; this test pins that guarantee so a
// future change (here or in the call helper) can't start leaking secrets.
func TestGetTxOutDoesNotLeakCredentials(t *testing.T) {
	const password = "s3cr3t-rpc-passw0rd"
	// Unroutable port so the request fails at dial, producing a transport error.
	c := NewClient("http://rpcuser:" + password + "@127.0.0.1:1/")
	_, err := c.GetTxOut(context.Background(), "aa", 0, false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), password, "RPC password must not appear in the error")
}

// TestChainSuccess confirms getblockchaininfo's chain field is returned.
func TestChainSuccess(t *testing.T) {
	c := newTestServer(t, `{"result":{"chain":"main","blocks":800000},"error":null,"id":"go-verifier-api"}`)
	chain, err := c.Chain(context.Background())
	require.NoError(t, err)
	require.Equal(t, "main", chain)
}

// TestChainRPCError confirms an RPC error surfaces as ErrFetchChainInfo so the
// caller keeps the request path fail-closed and retries (503).
func TestChainRPCError(t *testing.T) {
	c := newTestServer(t, `{"result":null,"error":{"code":-28,"message":"loading block index"},"id":"go-verifier-api"}`)
	_, err := c.Chain(context.Background())
	require.ErrorIs(t, err, ErrFetchChainInfo)
}

// TestChainEmptyResult confirms a null result (no chain readable) is an error,
// not a silent empty chain that would spuriously match nothing.
func TestChainEmptyResult(t *testing.T) {
	c := newTestServer(t, `{"result":null,"error":null,"id":"go-verifier-api"}`)
	_, err := c.Chain(context.Background())
	require.ErrorIs(t, err, ErrFetchChainInfo)
}

// TestChainTransportError confirms a transport failure surfaces as
// ErrFetchChainInfo.
func TestChainTransportError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1/")
	_, err := c.Chain(context.Background())
	require.ErrorIs(t, err, ErrFetchChainInfo)
}
