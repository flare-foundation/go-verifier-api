// Package nodechain reads a settling PMW batch transaction from a Bitcoin node,
// addressed by its txid.
//
// It is the CSP counterpart to package indexerdb. The request carries the
// settling txid, so this package never searches: it asks the node for one
// transaction by id, which -txindex=1 answers directly. No index participates,
// and nothing here has to be indexed, backfilled, or kept in step with the
// chain.
//
// The txid is a LOCATOR, not an identifier. Nothing in this package establishes
// that the transaction it returns is the right one — that is the caller's
// business, and the caller has the on-chain instruction it must match. What
// this package does guarantee is that the transaction is CONFIRMED ON THE
// ACTIVE CHAIN, which a txid lookup alone does not: a node with -txindex=1
// keeps serving a transaction whose block was reorged away, and reporting that
// as settled would prove a payment the chain no longer carries.
package nodechain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/batchtx"
)

// rpcTxNotFound is bitcoind's error code for an unknown transaction. It is the
// ordinary answer for a batch that has not been broadcast or has been dropped,
// so it is reported as "no batch" rather than as a failure.
const rpcTxNotFound = -5

// verbosityPrevout is the getrawtransaction verbosity that adds each input's
// prevout (value and scriptPubKey) and the transaction's own fee. It requires
// Bitcoin Core 25 or newer.
const verbosityPrevout = 2

const (
	requestTimeout = 10 * time.Second
	// maxResponseSize caps a decoded JSON-RPC response so a hostile or broken node
	// cannot exhaust memory. A verbose transaction is a few KB; 4 MB is ample.
	maxResponseSize = 4 << 20
	// maxConnsPerHost / maxIdleConnsPerHost bound connections to the node so a
	// request burst cannot exhaust its RPC pool or our sockets.
	maxConnsPerHost     = 16
	maxIdleConnsPerHost = 8
	// maxConcurrentRPC is the hard in-flight cap; beyond it calls fail fast
	// (retryable) rather than piling up goroutines/memory.
	maxConcurrentRPC = 64
)

// ErrNodeUnavailable marks a Bitcoin-node fault that is NOT a definitive "no such
// transaction": transport failure, an over-cap/malformed response, an in-flight
// overload, or a node/RPC-level error. It is retryable (mapped to 503), so an
// outage never turns into a false "not found" (status 2) or a 500.
var ErrNodeUnavailable = errors.New("bitcoin node unavailable")

// Repo reads batch transactions from a Bitcoin node over JSON-RPC.
type Repo struct {
	url              string
	http             *http.Client
	minConfirmations int64
	sem              chan struct{} // in-flight RPC cap; fail-fast when full
}

// NewRepo constructs a Repo against a Bitcoin JSON-RPC endpoint. minConfirmations
// is the depth floor a settling block must meet; a batch shallower than it reads
// as not-yet-confirmed (nil).
func NewRepo(url string, minConfirmations uint64) *Repo {
	return &Repo{
		url: url,
		http: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     maxConnsPerHost,
				MaxIdleConnsPerHost: maxIdleConnsPerHost,
			},
		},
		minConfirmations: int64(minConfirmations),
		sem:              make(chan struct{}, maxConcurrentRPC),
	}
}

type scriptPubKey struct {
	Hex       string   `json:"hex"`
	Address   string   `json:"address"`
	Addresses []string `json:"addresses"` // pre-22 nodes
}

// address returns the single address this script pays, preferring the modern
// field and falling back to the legacy array.
func (s scriptPubKey) address() string {
	if s.Address != "" {
		return s.Address
	}
	if len(s.Addresses) == 1 {
		return s.Addresses[0]
	}
	return ""
}

type txOut struct {
	Value        json.Number  `json:"value"`
	N            uint32       `json:"n"`
	ScriptPubKey scriptPubKey `json:"scriptPubKey"`
}

type txIn struct {
	Txid    string `json:"txid"`
	Vout    uint32 `json:"vout"`
	Prevout *struct {
		Value        json.Number  `json:"value"`
		ScriptPubKey scriptPubKey `json:"scriptPubKey"`
	} `json:"prevout"`
}

type rawTx struct {
	Txid      string      `json:"txid"`
	BlockHash string      `json:"blockhash"`
	Fee       json.Number `json:"fee"`
	Vin       []txIn      `json:"vin"`
	Vout      []txOut     `json:"vout"`
}

type blockHeader struct {
	Height        uint64 `json:"height"`
	Time          uint64 `json:"time"`
	Confirmations int64  `json:"confirmations"`
}

// Batch returns the confirmed transaction with the given txid (display order),
// or nil when there is no such transaction on the active chain.
//
// Nil is the answer for every ordinary way a settlement can be absent — never
// broadcast, still in the mempool, or in a block that was reorged away — and
// the caller turns it into a not-found attestation. An error means the node
// could not be consulted, which is a different thing and must not be reported
// as a payment that did not happen.
func (r *Repo) Batch(ctx context.Context, txid string) (*batchtx.BatchTx, error) {
	var tx rawTx
	err := r.call(ctx, "getrawtransaction", []any{txid, verbosityPrevout}, &tx)
	if err != nil {
		if isTxNotFound(err) {
			return nil, nil
		}
		return nil, asNodeUnavailable(err)
	}
	if tx.BlockHash == "" {
		// Known to the node but unconfirmed: in the mempool, or in no block yet.
		return nil, nil
	}

	// The block must be on the ACTIVE chain. getrawtransaction still resolves a
	// transaction whose block was reorged away, and its blockhash still names
	// that orphaned block, so the header's own confirmations are what separate
	// a settled batch from one the chain has abandoned.
	var header blockHeader
	if err := r.call(ctx, "getblockheader", []any{tx.BlockHash}, &header); err != nil {
		if isTxNotFound(err) {
			return nil, nil
		}
		return nil, asNodeUnavailable(err)
	}
	// Require the configured confirmation-depth floor. A settlement proof closes a
	// redemption, so a shallow block is reorg-fragile; below the floor the batch is
	// treated as not-yet-confirmed (not an error).
	if header.Confirmations < r.minConfirmations {
		return nil, nil
	}

	outputs, outSum, err := outputsOf(tx)
	if err != nil {
		return nil, asNodeUnavailable(err)
	}
	fee, err := feeOf(tx, outSum)
	if err != nil {
		return nil, asNodeUnavailable(err)
	}
	return &batchtx.BatchTx{
		Txid:               tx.Txid,
		BlockNumber:        header.Height,
		BlockTimestamp:     header.Time,
		Fee:                fee,
		Outputs:            outputs,
		AnchorInputAddress: anchorInputAddress(tx),
		Confirmations:      header.Confirmations,
	}, nil
}

// outputsOf converts the node's outputs to satoshi-valued outputs in n order,
// returning their sum.
func outputsOf(tx rawTx) ([]batchtx.Output, int64, error) {
	outputs := make([]batchtx.Output, len(tx.Vout))
	var sum int64
	for i, o := range tx.Vout {
		script, err := hex.DecodeString(o.ScriptPubKey.Hex)
		if err != nil {
			return nil, 0, fmt.Errorf("output %s:%d has invalid script hex: %w", tx.Txid, o.N, err)
		}
		sats, err := batchtx.SatsFromBTC(o.Value.String())
		if err != nil {
			return nil, 0, fmt.Errorf("output %s:%d: %w", tx.Txid, o.N, err)
		}
		outputs[i] = batchtx.Output{PkScript: script, Value: sats}
		sum += sats
	}
	return outputs, sum, nil
}

// feeOf prefers the node's own fee field and falls back to summing the inputs'
// prevout values, so a node that omits the field (no undo data) still yields a
// fee rather than a silent zero.
func feeOf(tx rawTx, outSum int64) (int64, error) {
	if s := tx.Fee.String(); s != "" && s != "0" {
		sats, err := batchtx.SatsFromBTC(s)
		if err != nil {
			return 0, fmt.Errorf("transaction %s reports an invalid fee: %w", tx.Txid, err)
		}
		return sats, nil
	}
	var inSum int64
	for i, in := range tx.Vin {
		if in.Prevout == nil {
			return 0, fmt.Errorf("transaction %s reports neither a fee nor a prevout for input %d", tx.Txid, i)
		}
		sats, err := batchtx.SatsFromBTC(in.Prevout.Value.String())
		if err != nil {
			return 0, fmt.Errorf("input %d of %s: %w", i, tx.Txid, err)
		}
		inSum += sats
	}
	fee := inSum - outSum
	if fee < 0 {
		return 0, fmt.Errorf("transaction %s spends more than it holds (in %d < out %d)", tx.Txid, inSum, outSum)
	}
	return fee, nil
}

// anchorInputAddress returns the address paid by the output input[0] spends,
// or "" when the node did not report it.
func anchorInputAddress(tx rawTx) string {
	if len(tx.Vin) == 0 || tx.Vin[0].Prevout == nil {
		return ""
	}
	return tx.Vin[0].Prevout.ScriptPubKey.address()
}

// OutputAddress resolves the address paid by the output at (txid, vout).
//
// It turns a chain's genesis anchor outpoint — which the channel names, and
// which the first batch on that chain has already spent — into the anchor
// address every batch on the chain reuses. Because the outpoint may be spent,
// the UTXO set cannot answer it; the transaction that created it can, and
// -txindex=1 is what makes that lookup possible.
func (r *Repo) OutputAddress(ctx context.Context, txid string, vout uint32) (string, error) {
	var tx rawTx
	err := r.call(ctx, "getrawtransaction", []any{txid, verbosityPrevout}, &tx)
	if err != nil {
		// The genesis anchor is registry-guaranteed to exist, so even "no such
		// transaction" (-5) is a node fault here — missing -txindex, an unsynced
		// node, or the wrong chain — never a legitimate absence. Fail closed so a
		// wrong-chain/misconfigured node cannot mint a false not-found.
		return "", asNodeUnavailable(err)
	}
	for _, o := range tx.Vout {
		if o.N == vout {
			if addr := o.ScriptPubKey.address(); addr != "" {
				return addr, nil
			}
			return "", fmt.Errorf("%w: anchor output %s:%d pays no single address", ErrNodeUnavailable, txid, vout)
		}
	}
	return "", fmt.Errorf("%w: anchor output %s:%d not present in the transaction", ErrNodeUnavailable, txid, vout)
}

// Chain returns the network the node serves ("main", "test", "signet" or
// "regtest") via getblockchaininfo, so the verifier can pin the node to the chain
// its parameters expect. Any fault is wrapped in ErrNodeUnavailable (retryable).
func (r *Repo) Chain(ctx context.Context) (string, error) {
	var info struct {
		Chain string `json:"chain"`
	}
	if err := r.call(ctx, "getblockchaininfo", []any{}, &info); err != nil {
		return "", asNodeUnavailable(err)
	}
	return info.Chain, nil
}

// rpcError is a bitcoind JSON-RPC error, kept typed so callers can tell an
// unknown transaction from a node that could not be reached.
type rpcError struct {
	Code    int
	Message string
}

func (e *rpcError) Error() string { return fmt.Sprintf("%s (code %d)", e.Message, e.Code) }

// asRPCError reports whether err is an *rpcError, assigning it to target.
func asRPCError(err error, target **rpcError) bool {
	e, ok := err.(*rpcError) //nolint:errorlint // the call helper returns this type directly, unwrapped
	if ok {
		*target = e
	}
	return ok
}

// isTxNotFound reports whether err is bitcoind's "no such transaction" — the one
// error that means a definite absence (→ not-found), not a node fault.
func isTxNotFound(err error) bool {
	var rpcErr *rpcError
	return asRPCError(err, &rpcErr) && rpcErr.Code == rpcTxNotFound
}

// asNodeUnavailable marks a non-not-found error retryable if it is not already:
// a raw RPC-level error or unusable node data is as much a node fault as a
// transport failure, and must map to 503 rather than a false not-found or a 500.
func asNodeUnavailable(err error) error {
	if errors.Is(err, ErrNodeUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrNodeUnavailable, err)
}

func (r *Repo) call(ctx context.Context, method string, params []any, out any) error {
	// Fail fast when already at the in-flight cap so a flood cannot grow
	// goroutines/memory without bound; the miss is retryable.
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	default:
		return fmt.Errorf("%w: too many concurrent node RPC calls", ErrNodeUnavailable)
	}

	body, err := json.Marshal(map[string]any{"jsonrpc": "1.0", "id": "verifier", "method": method, "params": params})
	if err != nil {
		return fmt.Errorf("encoding %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: calling %s: %w", ErrNodeUnavailable, method, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response; a close error changes nothing

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	// Bound the decoded body so an over-large or hostile response cannot exhaust
	// memory; a body over the cap is a node fault, not a payment outcome.
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&envelope); err != nil {
		return fmt.Errorf("%w: decoding %s response: %w", ErrNodeUnavailable, method, err)
	}
	if envelope.Error != nil {
		return &rpcError{Code: envelope.Error.Code, Message: envelope.Error.Message}
	}
	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("%w: decoding %s result: %w", ErrNodeUnavailable, method, err)
	}
	return nil
}
