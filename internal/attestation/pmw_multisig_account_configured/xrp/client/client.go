package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/type"
)

const (
	chainMaxAttemps     = 2
	chainRetryDelay     = 500 * time.Millisecond
	chainRequestTimeout = 4 * time.Second
)

type Client struct {
	client *http.Client
	url    string
}

type request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func NewClient(url string) *Client {
	return &Client{
		client: &http.Client{},
		url:    url,
	}
}

func (c *Client) doRequest(ctx context.Context, request request) ([]byte, error) {
	getReq, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(getReq)
	req, err := http.NewRequest("POST", c.url, buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, chainRequestTimeout)
	defer cancel()

	req = req.WithContext(ctxWithTimeout)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return resBody, nil
}

func (c *Client) doRequestWithRetry(ctx context.Context, request request) ([]byte, error) {
	return coreutil.Retry(
		chainMaxAttemps,
		chainRetryDelay,
		func() ([]byte, error) {
			return c.doRequest(ctx, request)
		}, nil)
}

func (c *Client) GetAccountInfo(ctx context.Context, account string) (*types.AccountInfoResponse, error) {
	request := request{
		Method: "account_info",
		Params: []interface{}{
			map[string]interface{}{
				"account":      account,
				"ledger_index": "validated",
				"signer_lists": true,
			},
		},
	}
	var accountInfo types.AccountInfoResponse
	raw, err := c.doRequestWithRetry(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}
	if err := json.Unmarshal(raw, &accountInfo); err != nil {
		return nil, fmt.Errorf("failed to parse account info: %w", err)
	}
	if accountInfo.Result.Status != "success" {
		return nil, errors.New("xrp rpc returned non-success status")
	}

	return &accountInfo, nil
}
