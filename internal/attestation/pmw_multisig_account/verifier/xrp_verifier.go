package verifier

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/xrpscan/xrpl-go"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	sequence, ok, err := x.verifyMultisigConfiguration(req)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	if ok {
		return connector.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(attestationtypes.PMWMultisigAccountStatusOK),
			Sequence: sequence,
		}, nil
	}
	return connector.IPMWMultisigAccountConfiguredResponseBody{
		Status:   uint8(attestationtypes.PMWMultisigAccountStatusERROR),
		Sequence: 0,
	}, nil
}

type signerList struct {
	SignerQuorum  uint64 `json:"SignerQuorum"`
	SignerEntries []struct {
		SignerEntry struct {
			Account      string `json:"Account"`
			SignerWeight uint16 `json:"SignerWeight"`
		} `json:"SignerEntry"`
	} `json:"SignerEntries"`
}

type accountInfoResponse struct {
	Result struct {
		AccountData struct {
			Account     string       `json:"Account"`
			Sequence    uint64       `json:"Sequence"`
			RegularKey  string       `json:"RegularKey,omitempty"`
			SignerLists []signerList `json:"signer_lists"`
		} `json:"account_data"`
		AccountFlags struct {
			DisableMasterKey      bool `json:"disableMasterKey"`
			DepositAuth           bool `json:"depositAuth"`
			RequireDestinationTag bool `json:"requireDestinationTag"`
			DisallowIncomingXRP   bool `json:"disallowIncomingXRP"`
		} `json:"account_flags"`
	} `json:"result"`
	Status string `json:"status"`
}

func (x *XRPVerifier) verifyMultisigConfiguration(req connector.IPMWMultisigAccountConfiguredRequestBody) (uint64, bool, error) {
	client := xrpl.NewClient(xrpl.ClientConfig{URL: x.config.RPCURL})
	request := xrpl.BaseRequest{
		"command":      "account_info",
		"account":      req.WalletAddress,
		"ledger_index": "validated",
		"signer_lists": true,
	}
	rpcResp, err := client.Request(request)
	if err != nil {
		return 0, false, fmt.Errorf("account_info request failed: %w", err)
	}

	var accountInfo accountInfoResponse
	raw, err := json.Marshal(rpcResp)
	if err != nil {
		return 0, false, fmt.Errorf("marshal rpc response: %w", err)
	}
	if err := json.Unmarshal(raw, &accountInfo); err != nil {
		return 0, false, fmt.Errorf("decode rpc response: %w", err)
	}
	if accountInfo.Status != "success" {
		return 0, false, fmt.Errorf("xrp rpc returned non-success status: %s", accountInfo.Status)
	}
	if len(accountInfo.Result.AccountData.SignerLists) == 0 {
		return 0, false, nil
	}

	signersValid := false
	for _, signerList := range accountInfo.Result.AccountData.SignerLists {
		valid := x.verifySignerList(signerList, req)
		signersValid = signersValid || valid
	}
	if !signersValid {
		return 0, false, nil
	}

	flags := accountInfo.Result.AccountFlags
	if !flags.DisableMasterKey {
		return 0, false, nil
	}
	if flags.DepositAuth {
		return 0, false, nil
	}
	if flags.RequireDestinationTag {
		return 0, false, nil
	}
	if flags.DisallowIncomingXRP {
		return 0, false, nil
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		return 0, false, nil
	}
	return accountInfo.Result.AccountData.Sequence, true, nil
}

func (x *XRPVerifier) verifySignerList(signerList signerList, req connector.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make([]string, 0, len(req.PublicKeys))
	for _, pk := range req.PublicKeys {
		addrStr, _ := address.PubToAddress(hex.EncodeToString(pk))
		expectedAccounts = append(expectedAccounts, addrStr)
	}
	actualAccounts := make(map[string]uint16)
	for _, entry := range signerList.SignerEntries {
		actualAccounts[entry.SignerEntry.Account] = entry.SignerEntry.SignerWeight
		if entry.SignerEntry.SignerWeight != 1 {
			return false
		}
	}
	if len(actualAccounts) != len(expectedAccounts) {
		return false
	}
	for _, acc := range expectedAccounts {
		if _, found := actualAccounts[acc]; !found {
			return false
		}
	}
	return signerList.SignerQuorum == req.Threshold
}
