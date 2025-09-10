package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	client         *http.Client
	url            string
	nRetries       int
	requestTimeout time.Duration
}

type request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// Derived from https://xrpl.org/docs/references/http-websocket-apis/public-api-methods/account-methods/account_info
type AccountInfoResponse struct {
	Result struct {
		AccountData struct {
			Account     string       `json:"Account"`
			Sequence    uint64       `json:"Sequence"`
			RegularKey  string       `json:"RegularKey,omitempty"`
			SignerLists []SignerList `json:"signer_lists"`
		} `json:"account_data"`
		AccountFlags struct {
			DisableMasterKey      bool `json:"disableMasterKey"`
			DepositAuth           bool `json:"depositAuth"`
			RequireDestinationTag bool `json:"requireDestinationTag"`
			DisallowIncomingXRP   bool `json:"disallowIncomingXRP"`
		} `json:"account_flags"`
		Status string `json:"status"`
	} `json:"result"`
}

type SignerList struct {
	SignerQuorum  uint64 `json:"SignerQuorum"`
	SignerEntries []struct {
		SignerEntry struct {
			Account      string `json:"Account"`
			SignerWeight uint16 `json:"SignerWeight"`
		} `json:"SignerEntry"`
	} `json:"SignerEntries"`
}

func NewClient(url string, nRetries int, requestTimeout time.Duration) *Client {
	return &Client{
		client:         &http.Client{},
		url:            url,
		nRetries:       nRetries,
		requestTimeout: requestTimeout,
	}
}

func (c *Client) get(ctx context.Context, request request) ([]byte, error) {
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
	req = req.WithContext(ctx)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	req = req.WithContext(ctxWithTimeout)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("error response status")
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return resBody, nil
}

func (c *Client) getWithRetry(ctx context.Context, request request) ([]byte, error) {
	for i := 0; i < c.nRetries; i++ {
		res, err := c.get(ctx, request)
		if err == nil {
			return res, nil
		}
	}
	return nil, fmt.Errorf("failed to get response after %d retries", c.nRetries)
}

func (c *Client) GetAccountInfo(ctx context.Context, account string) (*AccountInfoResponse, error) {
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
	raw, err := c.getWithRetry(ctx, request)
	if err != nil {
		return nil, err
	}
	var accountInfo AccountInfoResponse
	if err := json.Unmarshal(raw, &accountInfo); err != nil {
		return nil, err
	}

	if accountInfo.Result.Status != "success" {
		return nil, errors.New("xrp rpc returned non-success status")
	}

	return &accountInfo, nil
}
