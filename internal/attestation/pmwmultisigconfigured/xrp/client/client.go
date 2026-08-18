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
	// ErrRPCNonSuccess indicates the XRP RPC returned a non-success status (e.g. account not found).
	ErrRPCNonSuccess = errors.New("XRP RPC returned non-success status")
	// ErrFetchAccountInfo indicates a network/transport failure when fetching account info.
	ErrFetchAccountInfo = errors.New("cannot get account info")
	// ErrFetchServerInfo indicates a failure fetching server info (used for the network pin).
	ErrFetchServerInfo = errors.New("cannot get server info")
)

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
		return nil, fmt.Errorf("%w for account %s: %s", ErrRPCNonSuccess, account, resp.Message.Result.Status)
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
