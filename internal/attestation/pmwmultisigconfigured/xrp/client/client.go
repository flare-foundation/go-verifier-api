package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/call"
	"github.com/flare-foundation/go-flare-common/pkg/retry"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/types"
)

var (
	// ErrRPCNonSuccess indicates the XRP RPC returned a DETERMINISTIC non-success
	// status — a real negative answer such as actNotFound. Terminal: the account
	// state is settled, so it maps to a rejection, not a retry.
	ErrRPCNonSuccess = errors.New("XRP RPC returned non-success status")
	// ErrRPCTransient indicates the node is momentarily unable to answer (not synced,
	// too busy, rate-limited, ...) rather than a deterministic negative. Retryable:
	// a transient node state must not mint a terminal rejection.
	ErrRPCTransient = errors.New("XRP RPC temporarily unavailable")
	// ErrFetchAccountInfo indicates a network/transport failure when fetching account info.
	ErrFetchAccountInfo = errors.New("cannot get account info")
	// ErrFetchServerInfo indicates a failure fetching server info (used for the network pin).
	ErrFetchServerInfo = errors.New("cannot get server info")
)

// transientRPCStatuses are XRPL result statuses that mean "the node cannot answer
// right now" — retryable, not a deterministic negative. Anything not listed here
// (notably actNotFound, and malformed-request statuses) is treated as a terminal
// non-success. Extend this set as new transient statuses are encountered.
var transientRPCStatuses = map[string]bool{
	"noNetwork":        true, // not synced to the network
	"noCurrent":        true, // no current ledger available
	"noClosed":         true, // no closed ledger available
	"tooBusy":          true, // server overloaded
	"slowDown":         true, // rate limited
	"internal":         true, // internal server error
	"amendmentBlocked": true, // node needs upgrade; another node can answer
}

// rpcStatusError maps a non-success XRPL status to the right sentinel: retryable
// for transient node states, terminal otherwise.
func rpcStatusError(account, status string) error {
	if transientRPCStatuses[status] {
		return fmt.Errorf("%w for account %s: %s", ErrRPCTransient, account, status)
	}
	return fmt.Errorf("%w for account %s: %s", ErrRPCNonSuccess, account, status)
}

const (
	chainMaxAttempts           = 2
	chainRetryDelay            = 500 * time.Millisecond
	chainRequestTimeout        = 4 * time.Second
	maxAccountInfoResponseSize = 256 * 1024 // 256 KB
	maxServerInfoResponseSize  = 256 * 1024 // 256 KB
)

type Client struct {
	url string
}

type request struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

func NewClient(url string) *Client {
	return &Client{url: url}
}

func (c *Client) FetchAccountInfo(ctx context.Context, account string) (*types.AccountInfoResponse, error) {
	req := request{
		Method: "account_info",
		Params: []any{
			map[string]any{
				"account":      account,
				"ledger_index": "validated",
				"signer_lists": true,
			},
		},
	}
	resp, err := call.PostWithRetry[request, types.AccountInfoResponse](
		ctx,
		c.url,
		call.NoAPIKey,
		req,
		call.Params{
			Timeout:         chainRequestTimeout,
			MaxResponseSize: maxAccountInfoResponseSize,
		},
		nil,
		retry.Params{
			MaxAttempts: chainMaxAttempts,
			Delay:       chainRetryDelay,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetchAccountInfo, err)
	}
	if resp.Message.Result.Status != "success" {
		return nil, rpcStatusError(account, resp.Message.Result.Status)
	}

	return resp.Message, nil
}

// NetworkID fetches the node's network_id via server_info, used to pin the source
// network. present is false when the node does not report one (an older node); an
// unreachable node or non-success status returns an error.
func (c *Client) NetworkID(ctx context.Context) (networkID uint32, present bool, err error) {
	req := request{
		Method: "server_info",
		Params: []any{map[string]any{}},
	}
	resp, err := call.PostWithRetry[request, types.ServerInfoResponse](
		ctx,
		c.url,
		call.NoAPIKey,
		req,
		call.Params{
			Timeout:         chainRequestTimeout,
			MaxResponseSize: maxServerInfoResponseSize,
		},
		nil,
		retry.Params{
			MaxAttempts: chainMaxAttempts,
			Delay:       chainRetryDelay,
		},
	)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %w", ErrFetchServerInfo, err)
	}
	if resp.Message.Result.Status != "success" {
		return 0, false, fmt.Errorf("%w: status %q", ErrFetchServerInfo, resp.Message.Result.Status)
	}
	if !resp.Message.Result.Info.NetworkID.Present {
		return 0, false, nil
	}
	return resp.Message.Result.Info.NetworkID.Value, true, nil
}
