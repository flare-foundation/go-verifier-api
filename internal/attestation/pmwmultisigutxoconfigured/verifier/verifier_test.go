package verifier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	btcverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// stubNode starts an httptest server that answers every JSON-RPC call with a
// getblockchaininfo result naming the given chain, and returns its URL. It lets
// the wiring test drive the full constructor path (which calls VerifyNetwork ->
// getblockchaininfo) without a live Bitcoin node.
func stubNode(t *testing.T, chain string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"chain":"` + chain + `","blocks":100},"error":null,"id":"go-verifier-api"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// utxoConfig builds a minimal PMWMultisigUtxoConfig for the given source, network
// override, and node URL.
func utxoConfig(source config.SourceName, network, rpcURL string) *config.PMWMultisigUtxoConfig {
	return &config.PMWMultisigUtxoConfig{
		EncodedAndABI: config.EncodedAndABI{
			SourceIDPair: config.SourceIDEncodedPair{SourceID: source},
		},
		BtcNetwork:   network,
		SourceRPCURL: rpcURL,
	}
}

func TestNewVerifier(t *testing.T) {
	t.Run("unsupported source errors before any node call", func(t *testing.T) {
		for _, src := range []config.SourceName{"UNSUPPORTED_SOURCE", "", config.SourceTEE} {
			v, err := NewVerifier(utxoConfig(src, "", ""))
			require.Nil(t, v)
			require.ErrorContains(t, err, "no verifier for sourceID")
		}
	})

	t.Run("supported sources resolve against a matching node", func(t *testing.T) {
		// BTC_NETWORK=regtest overrides the source-implied default, so one stub
		// chain ("regtest") satisfies the network pin for both sources.
		url := stubNode(t, "regtest")
		for _, src := range []config.SourceName{config.SourceBTC, config.SourceTestBTC} {
			v, err := NewVerifier(utxoConfig(src, "regtest", url))
			require.NoError(t, err)
			require.NotNil(t, v)
		}
	})

	t.Run("wrong-chain node fails construction", func(t *testing.T) {
		// Params resolve to regtest, but the node reports mainnet: VerifyNetwork
		// returns ErrNetworkMismatch, which the constructor propagates.
		url := stubNode(t, "main")
		v, err := NewVerifier(utxoConfig(config.SourceBTC, "regtest", url))
		require.Nil(t, v)
		require.ErrorIs(t, err, btcverifier.ErrNetworkMismatch)
	})

	t.Run("unknown BTC_NETWORK fails construction", func(t *testing.T) {
		v, err := NewVerifier(utxoConfig(config.SourceBTC, "nope", ""))
		require.Nil(t, v)
		require.Error(t, err)
	})
}
