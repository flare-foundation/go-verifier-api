package nodechain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeNode is a bitcoind stand-in that answers the two methods this package
// calls, so the reorg and not-found paths can be exercised without a chain.
type fakeNode struct {
	tx        json.RawMessage
	txErrCode int
	header    json.RawMessage
	headerErr int
	calls     []string
}

func (f *fakeNode) serve() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.calls = append(f.calls, req.Method)

		write := func(result json.RawMessage, code int) {
			w.Header().Set("Content-Type", "application/json")
			if code != 0 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"result": nil,
					"error":  map[string]any{"code": code, "message": "stub failure"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
		}
		switch req.Method {
		case "getrawtransaction":
			write(f.tx, f.txErrCode)
		case "getblockheader":
			write(f.header, f.headerErr)
		default:
			write(nil, -32601)
		}
	}))
}

// confirmedTxJSON is a two-output batch spending one anchor input, in the shape
// getrawtransaction returns at verbosity 2.
const confirmedTxJSON = `{
  "txid": "aa11",
  "blockhash": "beef",
  "fee": 0.00001,
  "vin": [{"txid":"parent","vout":0,
           "prevout":{"value":0.001,"scriptPubKey":{"hex":"0014aa","address":"bcrt1anchor"}}}],
  "vout": [{"value":0.0005,"n":0,"scriptPubKey":{"hex":"0014bb","address":"bcrt1out0"}},
           {"value":0.0004,"n":1,"scriptPubKey":{"hex":"0014cc","address":"bcrt1out1"}}]
}`

func TestBatchReadsConfirmedTransaction(t *testing.T) {
	f := &fakeNode{
		tx:     json.RawMessage(confirmedTxJSON),
		header: json.RawMessage(`{"height": 812345, "time": 1700000000, "confirmations": 7}`),
	}
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, uint64(812345), got.BlockNumber)
	require.Equal(t, uint64(1700000000), got.BlockTimestamp)
	require.Equal(t, int64(1000), got.Fee) // 0.00001 BTC
	require.Equal(t, int64(7), got.Confirmations)
	require.Equal(t, "bcrt1anchor", got.AnchorInputAddress)
	require.Len(t, got.Outputs, 2)
	require.Equal(t, int64(50000), got.Outputs[0].Value)
	require.Equal(t, []byte{0x00, 0x14, 0xbb}, got.Outputs[0].PkScript)
}

// TestBatchEnforcesConfirmationDepth: a block shallower than the configured floor
// reads as not-yet-confirmed (nil), and one at the floor is accepted. This is the
// reorg-safety depth (spec §7): a settlement proof closes a redemption.
func TestBatchEnforcesConfirmationDepth(t *testing.T) {
	const floor = 6
	t.Run("below floor is not confirmed", func(t *testing.T) {
		f := &fakeNode{
			tx:     json.RawMessage(confirmedTxJSON),
			header: json.RawMessage(`{"height": 812345, "time": 1700000000, "confirmations": 5}`),
		}
		srv := f.serve()
		defer srv.Close()
		got, err := NewRepo(srv.URL, floor).Batch(context.Background(), "aa11")
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("at floor is confirmed", func(t *testing.T) {
		f := &fakeNode{
			tx:     json.RawMessage(confirmedTxJSON),
			header: json.RawMessage(`{"height": 812345, "time": 1700000000, "confirmations": 6}`),
		}
		srv := f.serve()
		defer srv.Close()
		got, err := NewRepo(srv.URL, floor).Batch(context.Background(), "aa11")
		require.NoError(t, err)
		require.NotNil(t, got)
	})
}

// The check this package exists for: -txindex keeps serving a transaction whose
// block was reorged away, and its blockhash still names that orphaned block. If
// the header's confirmations were not consulted, a batch the chain has
// abandoned would be reported as settled.
func TestBatchRefusesTransactionInReorgedBlock(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(confirmedTxJSON),
		// bitcoind reports -1 for a header that is not on the active chain.
		header: json.RawMessage(`{"height": 812345, "time": 1700000000, "confirmations": -1}`),
	}
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.Nil(t, got, "a transaction whose block left the active chain is not a settlement")
	require.Equal(t, []string{"getrawtransaction", "getblockheader"}, f.calls,
		"the header must actually be consulted; resolving the transaction is not enough")
}

func TestBatchTreatsUnconfirmedAsAbsent(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(`{"txid":"aa11","vin":[],"vout":[]}`)}
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, []string{"getrawtransaction"}, f.calls, "no block to ask about")
}

func TestBatchTreatsUnknownTxidAsAbsent(t *testing.T) {
	f := &fakeNode{txErrCode: rpcTxNotFound}
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "deadbeef")
	require.NoError(t, err)
	require.Nil(t, got)
}

// An unreachable node is not a payment that did not happen. Collapsing the two
// would let an outage mint negative evidence.
func TestBatchReportsNodeFailureAsError(t *testing.T) {
	f := &fakeNode{txErrCode: -8} // not rpcTxNotFound
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.Error(t, err)
	require.Nil(t, got)
}

// Nodes that omit the fee field (no undo data) must still yield a fee rather
// than a silent zero, since the fee lands in the attestation.
func TestBatchDerivesFeeFromPrevoutsWhenAbsent(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{
		  "txid": "aa11", "blockhash": "beef",
		  "vin": [{"prevout":{"value":0.001,"scriptPubKey":{"hex":"0014aa","address":"bcrt1anchor"}}}],
		  "vout": [{"value":0.0009,"n":0,"scriptPubKey":{"hex":"0014bb","address":"bcrt1out0"}}]
		}`),
		header: json.RawMessage(`{"height": 1, "time": 2, "confirmations": 3}`),
	}
	srv := f.serve()
	defer srv.Close()

	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(10000), got.Fee) // 0.001 - 0.0009 BTC
}

func TestOutputAddressResolvesSpentOutput(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(confirmedTxJSON)}
	srv := f.serve()
	defer srv.Close()

	addr, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 1)
	require.NoError(t, err)
	require.Equal(t, "bcrt1out1", addr)
}
