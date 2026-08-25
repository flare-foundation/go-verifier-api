package nodechain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestChainReadsNetwork confirms getblockchaininfo's chain field is returned for
// the startup network pin.
func TestChainReadsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "signet"}, "error": nil})
	}))
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Chain(context.Background())
	require.NoError(t, err)
	require.Equal(t, "signet", got)
}

// TestBatchTransportErrorIsNodeUnavailable: a node that cannot be reached yields
// the retryable ErrNodeUnavailable (→ 503), never a false not-found (nil) or a 500.
func TestBatchTransportErrorIsNodeUnavailable(t *testing.T) {
	got, err := NewRepo("http://127.0.0.1:1", 1).Batch(context.Background(), "aa11")
	require.Nil(t, got)
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestInFlightCapFailsFast: once the in-flight cap is full, a further call fails
// fast with ErrNodeUnavailable instead of piling up.
func TestInFlightCapFailsFast(t *testing.T) {
	r := NewRepo("http://127.0.0.1:1", 1)
	for range cap(r.sem) {
		r.sem <- struct{}{}
	}
	_, err := r.Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestOutputAddressFailsClosedOnMissingAnchor: the genesis anchor is
// registry-guaranteed, so a "no such transaction" (-5) here is a node fault
// (wrong chain / no -txindex), not absence — it must fail closed, never "".
func TestOutputAddressFailsClosedOnMissingAnchor(t *testing.T) {
	f := &fakeNode{txErrCode: rpcTxNotFound}
	srv := f.serve()
	defer srv.Close()
	addr, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 0)
	require.Equal(t, "", addr)
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestChainProbeHasDedicatedPool: the chain pin uses its own semaphore, so a
// transaction-call flood that fills the gettxout pool cannot starve the
// safety-critical probe.
func TestChainProbeHasDedicatedPool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "signet"}, "error": nil})
	}))
	defer srv.Close()
	r := NewRepo(srv.URL, 1)
	for range cap(r.sem) { // saturate the transaction pool
		r.sem <- struct{}{}
	}
	got, err := r.Chain(context.Background()) // still served from the reserved chain pool
	require.NoError(t, err)
	require.Equal(t, "signet", got)

	for range cap(r.chainSem) { // now saturate the chain pool too
		r.chainSem <- struct{}{}
	}
	_, err = r.Chain(context.Background())
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestBatchRejectsReturnedTxIDMismatch: a node that answers with a different
// transaction than requested is a fault (or hostile), not this settlement.
func TestBatchRejectsReturnedTxIDMismatch(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(`{"txid":"bb22","blockhash":"beef","vout":[]}`)}
	srv := f.serve()
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.Nil(t, got)
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestOutputsRejectOutOfOrderVout: the PMW grammar is positional, so outputs must
// be the exact sequence 0..N-1; a reordered/gapped/duplicated vout index is
// refused rather than silently shifting group interpretation.
func TestOutputsRejectOutOfOrderVout(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vin":[{"prevout":{"value":0.001,"scriptPubKey":{"hex":"0014aa","address":"a"}}}],
		  "vout":[{"value":0.0005,"n":1,"scriptPubKey":{"hex":"0014bb"}},
		          {"value":0.0004,"n":0,"scriptPubKey":{"hex":"0014cc"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "out of order")
}

// TestTransactionValueSumsCannotOverflow: summed output values are bounded by the
// money supply, so a hostile node cannot overflow int64 into a bogus fee.
func TestTransactionValueSumsCannotOverflow(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vout":[{"value":21000000,"n":0,"scriptPubKey":{"hex":"0014bb"}},
		          {"value":21000000,"n":1,"scriptPubKey":{"hex":"0014cc"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "money supply")
}

// TestOutputAddressUsesLegacyAddressesField: a pre-22 node reports a single
// address in the array `addresses` rather than the scalar `address`; the resolver
// must still return it.
func TestOutputAddressUsesLegacyAddressesField(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(`{"txid":"aa11","vout":[
	  {"value":0.0005,"n":0,"scriptPubKey":{"hex":"0014bb","addresses":["bcrt1legacy"]}}]}`)}
	srv := f.serve()
	defer srv.Close()
	addr, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 0)
	require.NoError(t, err)
	require.Equal(t, "bcrt1legacy", addr)
}

// TestOutputAddressFailsWhenOutputPaysNoSingleAddress: an output whose script pays
// zero or many addresses cannot be turned into the anchor address, so it fails
// closed rather than returning "".
func TestOutputAddressFailsWhenOutputPaysNoSingleAddress(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(`{"txid":"aa11","vout":[
	  {"value":0.0005,"n":0,"scriptPubKey":{"hex":"6a00","addresses":["a","b"]}}]}`)}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 0)
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "no single address")
}

// TestOutputAddressFailsWhenVoutAbsent: asking for a vout the transaction does not
// have is a node/registry fault, not an empty address.
func TestOutputAddressFailsWhenVoutAbsent(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(confirmedTxJSON)}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 9)
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "not present")
}

// TestOutputAddressRejectsReturnedTxIDMismatch: a node answering with a different
// transaction than requested is a fault, not this anchor.
func TestOutputAddressRejectsReturnedTxIDMismatch(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(`{"txid":"bb22","vout":[]}`)}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).OutputAddress(context.Background(), "aa11", 0)
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestBatchRejectsInvalidScriptHex: a vout whose scriptPubKey hex is not valid hex
// is corrupt node data, surfaced as a node fault rather than a settlement.
func TestBatchRejectsInvalidScriptHex(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vout":[{"value":0.0005,"n":0,"scriptPubKey":{"hex":"zzzz"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestBatchRejectsInvalidFee: a node-reported fee that is not a valid amount is
// corrupt data, not a zero fee.
func TestBatchRejectsInvalidFee(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef","fee":21000001,
		  "vout":[{"value":0.0005,"n":0,"scriptPubKey":{"hex":"0014bb"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestBatchFailsWhenNeitherFeeNorPrevout: with no fee field and an input carrying
// no prevout, the fee is unknowable — fail closed rather than emit a silent zero.
func TestBatchFailsWhenNeitherFeeNorPrevout(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vin":[{"txid":"parent","vout":0}],
		  "vout":[{"value":0.0005,"n":0,"scriptPubKey":{"hex":"0014bb"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "neither a fee nor a prevout")
}

// TestBatchFailsWhenOutputsExceedInputs: a transaction whose outputs exceed its
// summed prevouts implies a negative fee, which is corrupt data.
func TestBatchFailsWhenOutputsExceedInputs(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vin":[{"prevout":{"value":0.0001,"scriptPubKey":{"hex":"0014aa","address":"a"}}}],
		  "vout":[{"value":0.0009,"n":0,"scriptPubKey":{"hex":"0014bb"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.ErrorContains(t, err, "spends more than it holds")
}

// TestBatchOmitsAnchorInputAddressWhenNoPrevout: a confirmed transaction whose
// input carries no prevout yields an empty AnchorInputAddress (the fee still comes
// from the node's own fee field), not a failure.
func TestBatchOmitsAnchorInputAddressWhenNoPrevout(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef","fee":0.00001,
		  "vin":[{"txid":"parent","vout":0}],
		  "vout":[{"value":0.0005,"n":0,"scriptPubKey":{"hex":"0014bb","address":"o"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "", got.AnchorInputAddress)
}

// TestBatchTreatsMissingHeaderAsAbsent: getblockheader returning "no such block"
// (the block was reorged out from under us between the two calls) is an absence,
// not an error.
func TestBatchTreatsMissingHeaderAsAbsent(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(confirmedTxJSON), headerErr: rpcTxNotFound}
	srv := f.serve()
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestBatchReportsHeaderFaultAsError: a getblockheader fault that is not a
// not-found is a node outage, never a settlement that did not happen.
func TestBatchReportsHeaderFaultAsError(t *testing.T) {
	f := &fakeNode{tx: json.RawMessage(confirmedTxJSON), headerErr: -8}
	srv := f.serve()
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	// Must be the retryable sentinel (→ 503), never a false not-found or a 500.
	require.ErrorIs(t, err, ErrNodeUnavailable)
	require.Nil(t, got)
}

// TestCallRejectsOversizedResponse: a response larger than maxResponseSize is
// truncated by the read cap and fails to decode, surfacing as a node fault rather
// than exhausting memory. This proves the bound deterministically, not just under
// fuzzing.
func TestCallRejectsOversizedResponse(t *testing.T) {
	// A valid JSON prefix whose string value runs past the 4 MiB read cap, so the
	// LimitReader truncates it mid-token and the decode fails.
	huge := `{"result":{"txid":"` + strings.Repeat("a", maxResponseSize+(1<<20)) + `"},"error":null}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, huge)
	}))
	defer srv.Close()
	got, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.Nil(t, got)
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestBatchRejectsNegativeOutputValue: a negative output value is corrupt node
// data (SatsFromBTC rejects it), surfaced as a node fault rather than a batch.
func TestBatchRejectsNegativeOutputValue(t *testing.T) {
	f := &fakeNode{
		tx: json.RawMessage(`{"txid":"aa11","blockhash":"beef",
		  "vout":[{"value":-1,"n":0,"scriptPubKey":{"hex":"0014bb"}}]}`),
		header: json.RawMessage(`{"height":1,"time":2,"confirmations":6}`),
	}
	srv := f.serve()
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Batch(context.Background(), "aa11")
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// TestChainRejectsMalformedResult: a getblockchaininfo result that is not an
// object cannot be decoded into the chain field, so the pin fails closed rather
// than reading an empty chain.
func TestChainRejectsMalformedResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"result":"not-an-object","error":null}`)
	}))
	defer srv.Close()
	_, err := NewRepo(srv.URL, 1).Chain(context.Background())
	require.ErrorIs(t, err, ErrNodeUnavailable)
}

// FuzzNodeRPCEnvelope: an arbitrary JSON-RPC response body must never panic the
// client; every method returns either an absence or an error, never a crash. The
// response-size cap keeps allocation bounded.
func FuzzNodeRPCEnvelope(f *testing.F) {
	f.Add([]byte(`{"result":{"txid":"aa11"},"error":null}`))
	f.Add([]byte(`{"error":{"code":-5,"message":"x"}}`))
	f.Add([]byte(`{"result":{"chain":"signet"}}`))
	f.Add([]byte(`garbage`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, response []byte) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(response)
		}))
		defer srv.Close()
		r := NewRepo(srv.URL, 1)
		_, _ = r.Batch(context.Background(), "aa11")            // must not panic
		_, _ = r.OutputAddress(context.Background(), "aa11", 0) // must not panic
		_, _ = r.Chain(context.Background())                    // must not panic
	})
}
