package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/call"
	"github.com/flare-foundation/go-flare-common/pkg/retry"
)

// ErrGetTxOut indicates a failure when calling gettxout — mapped to 503 so the
// caller retries (or an operator fixes a config fault) rather than voting a false
// negative. It covers every failure this call can produce: network/transport,
// node warmup, an in-flight cap rejection, and ANY JSON-RPC error. The latter is
// deliberate: gettxout always receives a well-formed 64-hex txid (from [32]byte)
// and a typed uint32 vout, and a missing/out-of-range output returns a null
// result, not an error — so a parameter/deserialization error (-3/-8/-22) or a
// protocol/method error (-32601 etc.) can only mean a node/client problem, never
// bad caller data. All are therefore transient/retryable, not a 4xx.
var ErrGetTxOut = errors.New("cannot get transaction output")

// ErrFetchChainInfo indicates a transient failure reading the node's chain via
// getblockchaininfo (network/transport, node warmup) — the caller may retry.
var ErrFetchChainInfo = errors.New("cannot get blockchain info")

// errTooManyConcurrent is returned (wrapped in the caller's transient sentinel)
// when the in-flight RPC cap is hit, so a flood fails fast instead of piling up.
var errTooManyConcurrent = errors.New("too many concurrent bitcoin RPC calls")

const (
	chainMaxAttempts     = 2
	chainRetryDelay      = 500 * time.Millisecond
	chainRequestTimeout  = 4 * time.Second
	maxTxOutResponseSize = 64 * 1024 // 64 KB — a single UTXO view is small
	// maxChainInfoResponseSize bounds the getblockchaininfo response. The full
	// object (softfork/warning fields included) is a few KB; 64 KB is ample.
	maxChainInfoResponseSize = 64 * 1024

	// maxConnsPerHost caps simultaneous TCP connections to the Bitcoin node so a
	// request flood cannot exhaust the node's RPC connection pool or our sockets;
	// excess calls wait for a free connection (bounded by the per-call timeout).
	maxConnsPerHost = 16
	// maxIdleConnsPerHost keeps a small warm pool for connection reuse.
	maxIdleConnsPerHost = 8
	// maxConcurrentRPC is the hard in-flight cap across all RPCs on this client.
	// Beyond it, calls fail fast (503) instead of piling up goroutines/memory —
	// the outer bound; maxConnsPerHost bounds the actual sockets underneath.
	maxConcurrentRPC = 64
)

// Client is a thin Bitcoin Core JSON-RPC client. Authentication credentials, if
// any, are carried in the URL userinfo (http://user:pass@host:port), matching
// bitcoind's HTTP basic auth.
type Client struct {
	url       string
	transport http.RoundTripper // shared; bounds connections to the node
	sem       chan struct{}     // in-flight RPC cap; fail-fast when full
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
		transport: &http.Transport{
			MaxConnsPerHost:     maxConnsPerHost,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
		},
		sem: make(chan struct{}, maxConcurrentRPC),
	}
}

// acquire reserves an in-flight RPC slot, returning ok=false immediately when the
// client is already at maxConcurrentRPC so a flood cannot grow goroutines/memory
// without bound. The returned release frees the slot (call only when ok).
func (c *Client) acquire() (release func(), ok bool) {
	select {
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, true
	default:
		return nil, false
	}
}

// GetTxOut returns the unspent output at (txid, vout) via Bitcoin Core's
// gettxout. txid is the transaction id in display (big-endian) hex — the form
// bitcoind expects, so no byte reversal is applied. A nil result with a nil
// error means the output does not exist or is already spent (Bitcoin Core
// returns a null result). includeMempool selects whether unconfirmed spends are
// considered; anchor verification passes false so only confirmed outputs match.
func (c *Client) GetTxOut(ctx context.Context, txid string, vout uint32, includeMempool bool) (*GetTxOut, error) {
	release, ok := c.acquire()
	if !ok {
		return nil, fmt.Errorf("%w: %w", ErrGetTxOut, errTooManyConcurrent)
	}
	defer release()

	req := jsonRPCRequest{
		JSONRPC: "1.0",
		ID:      "go-verifier-api",
		Method:  "gettxout",
		Params:  []any{txid, vout, includeMempool},
	}
	resp, err := call.PostWithRetry[jsonRPCRequest, getTxOutResponse](
		ctx,
		c.url,
		call.NoAPIKey,
		req,
		call.Params{
			Timeout:         chainRequestTimeout,
			MaxResponseSize: maxTxOutResponseSize,
			Transport:       c.transport,
		},
		nil,
		retry.Params{
			MaxAttempts: chainMaxAttempts,
			Delay:       chainRetryDelay,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetTxOut, err)
	}
	if resp.Message.Error != nil {
		// Any RPC error here is a node/client fault, never bad caller data (see
		// ErrGetTxOut): treat every one as transient/retryable (503).
		rpcErr := resp.Message.Error
		return nil, fmt.Errorf("%w %s:%d (code %d): %s", ErrGetTxOut, txid, vout, rpcErr.Code, rpcErr.Message)
	}
	// A null result means the output is unspent-not-found or already spent.
	return resp.Message.Result, nil
}

// Chain returns the network the node serves ("main", "test", "signet" or
// "regtest") via getblockchaininfo, so the verifier can pin the node to the chain
// its parameters expect. Any transport or RPC failure is wrapped in
// ErrFetchChainInfo so the caller keeps the request path fail-closed and retries.
func (c *Client) Chain(ctx context.Context) (string, error) {
	release, ok := c.acquire()
	if !ok {
		return "", fmt.Errorf("%w: %w", ErrFetchChainInfo, errTooManyConcurrent)
	}
	defer release()

	req := jsonRPCRequest{
		JSONRPC: "1.0",
		ID:      "go-verifier-api",
		Method:  "getblockchaininfo",
		Params:  []any{},
	}
	resp, err := call.PostWithRetry[jsonRPCRequest, getBlockchainInfoResponse](
		ctx,
		c.url,
		call.NoAPIKey,
		req,
		call.Params{
			Timeout:         chainRequestTimeout,
			MaxResponseSize: maxChainInfoResponseSize,
			Transport:       c.transport,
		},
		nil,
		retry.Params{
			MaxAttempts: chainMaxAttempts,
			Delay:       chainRetryDelay,
		},
	)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFetchChainInfo, err)
	}
	if resp.Message.Error != nil {
		rpcErr := resp.Message.Error
		return "", fmt.Errorf("%w (code %d): %s", ErrFetchChainInfo, rpcErr.Code, rpcErr.Message)
	}
	if resp.Message.Result == nil {
		return "", fmt.Errorf("%w: empty result", ErrFetchChainInfo)
	}
	return resp.Message.Result.Chain, nil
}
