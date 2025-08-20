package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type XrpClient struct {
	client *http.Client
	url    string
}

type xrpRequest struct {
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

func NewXrpClient(url string) XrpClient {
	return XrpClient{
		client: &http.Client{},
		url:    url,
	}
}

func (c XrpClient) GetResponse(ctx context.Context, request xrpRequest) ([]byte, error) {
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("error response status")
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return resBody, nil
}

func (c XrpClient) GetAccountInfo(ctx context.Context, account string) (AccountInfoResponse, error) {
	request := xrpRequest{
		Method: "account_info",
		Params: []interface{}{
			map[string]interface{}{
				"account":      account,
				"ledger_index": "validated",
				"signer_lists": true,
			},
		},
	}
	raw, err := c.GetResponse(ctx, request)
	if err != nil {
		return AccountInfoResponse{}, err
	}
	var accountInfo AccountInfoResponse
	if err := json.Unmarshal(raw, &accountInfo); err != nil {
		return AccountInfoResponse{}, err
	}

	if accountInfo.Result.Status != "success" {
		return AccountInfoResponse{}, errors.New("xrp rpc returned non-success status")
	}

	return accountInfo, nil
}
