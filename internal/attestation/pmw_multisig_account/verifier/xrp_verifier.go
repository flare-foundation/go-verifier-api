package verifier

import (
	"context"
	"encoding/hex"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
	client XrpClient
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	sequence, ok, err := x.verifyMultisigConfiguration(ctx, req)
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

func (x *XRPVerifier) verifyMultisigConfiguration(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (uint64, bool, error) {
	accountInfo, err := x.client.GetAccountInfo(ctx, req.WalletAddress)
	if err != nil {
		return 0, false, err
	}

	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	if len(accountInfo.Result.AccountData.SignerLists) == 0 {
		logger.Debug("Account validation failed: doesnt have signers")
		return 0, false, nil
	}

	signersValid := x.verifySignerList(accountInfo.Result.AccountData.SignerLists[0], req)
	if !signersValid {
		return 0, false, nil
	}

	flags := accountInfo.Result.AccountFlags
	if !flags.DisableMasterKey {
		logger.Debug("Account validation failed: master key is not disabled")
		return 0, false, nil
	}
	if flags.DepositAuth {
		logger.Debug("Account validation failed: deposit authorization is enabled")
		return 0, false, nil
	}
	if flags.RequireDestinationTag {
		logger.Debug("Account validation failed: destination tag is required")
		return 0, false, nil
	}
	if flags.DisallowIncomingXRP {
		logger.Debug("Account validation failed: incoming XRP is disallowed")
		return 0, false, nil
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		logger.Debug("Account validation failed: regular key is set")
		return 0, false, nil
	}
	return accountInfo.Result.AccountData.Sequence, true, nil
}

func (x *XRPVerifier) verifySignerList(signerList SignerList, req connector.IPMWMultisigAccountConfiguredRequestBody) bool {
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
