package client

import (
	"context"
	"fmt"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/call"
	"github.com/flare-foundation/go-flare-common/pkg/retry"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/types"
)

const (
	chainMaxAttempts           = 2
	chainRetryDelay            = 500 * time.Millisecond
	chainRequestTimeout        = 4 * time.Second
	maxAccountInfoResponseSize = 256 * 1024 // 256 KB
)

type Client struct {
	url string
}

type request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func NewClient(url string) *Client {
	return &Client{url: url}
}

func (c *Client) GetAccountInfo(ctx context.Context, account string) (*types.AccountInfoResponse, error) {
	req := request{
		Method: "account_info",
		Params: []interface{}{
			map[string]interface{}{
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
		return nil, fmt.Errorf("cannot get account info: %w", err)
	}
	if resp.Message.Result.Status != "success" {
		return nil, fmt.Errorf("XRP RPC returned non-success status for account %s: %s", account, resp.Message.Result.Status)
	}

	return resp.Message, nil
}
