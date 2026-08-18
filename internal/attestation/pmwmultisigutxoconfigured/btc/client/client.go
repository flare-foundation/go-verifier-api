package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/call"
	"github.com/flare-foundation/go-flare-common/pkg/retry"
)

// ErrGetTxOut indicates a transient failure (network/transport, node warmup, or
// an unclassified RPC error) when calling gettxout — the caller may retry.
var ErrGetTxOut = errors.New("cannot get transaction output")

// ErrRPCInvalidRequest indicates the node rejected the request deterministically
// (invalid/parse/type parameter error). Retrying cannot succeed, so it is kept
// distinct from the transient ErrGetTxOut so callers map it to a 4xx, not a 503.
var ErrRPCInvalidRequest = errors.New("bitcoin rpc rejected the request")

const (
	chainMaxAttempts     = 2
	chainRetryDelay      = 500 * time.Millisecond
	chainRequestTimeout  = 4 * time.Second
	maxTxOutResponseSize = 64 * 1024 // 64 KB — a single UTXO view is small
)

// isDeterministicRPCError reports whether a Bitcoin Core JSON-RPC error code is
// a permanent, request-level rejection (bad parameters / malformed request)
// rather than a transient condition. Codes are from Bitcoin Core's
// rpc/protocol.h. Anything not listed (e.g. -28 RPC_IN_WARMUP, -1 misc) is
// treated as transient and left retryable.
func isDeterministicRPCError(code int) bool {
	switch code {
	case -3, // RPC_TYPE_ERROR
		-8,     // RPC_INVALID_PARAMETER
		-22,    // RPC_DESERIALIZATION_ERROR (e.g. malformed txid)
		-32600, // RPC_INVALID_REQUEST
		-32601, // RPC_METHOD_NOT_FOUND
		-32602, // RPC_INVALID_PARAMS
		-32700: // RPC_PARSE_ERROR
		return true
	default:
		return false
	}
}

// Client is a thin Bitcoin Core JSON-RPC client. Authentication credentials, if
// any, are carried in the URL userinfo (http://user:pass@host:port), matching
// bitcoind's HTTP basic auth.
type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{url: url}
}

// GetTxOut returns the unspent output at (txid, vout) via Bitcoin Core's
// gettxout. txid is the transaction id in display (big-endian) hex — the form
// bitcoind expects, so no byte reversal is applied. A nil result with a nil
// error means the output does not exist or is already spent (Bitcoin Core
// returns a null result). includeMempool selects whether unconfirmed spends are
// considered; anchor verification passes false so only confirmed outputs match.
func (c *Client) GetTxOut(ctx context.Context, txid string, vout uint32, includeMempool bool) (*GetTxOut, error) {
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
		rpcErr := resp.Message.Error
		if isDeterministicRPCError(rpcErr.Code) {
			return nil, fmt.Errorf("%w %s:%d (code %d): %s", ErrRPCInvalidRequest, txid, vout, rpcErr.Code, rpcErr.Message)
		}
		return nil, fmt.Errorf("%w %s:%d (code %d): %s", ErrGetTxOut, txid, vout, rpcErr.Code, rpcErr.Message)
	}
	// A null result means the output is unspent-not-found or already spent.
	return resp.Message.Result, nil
}
