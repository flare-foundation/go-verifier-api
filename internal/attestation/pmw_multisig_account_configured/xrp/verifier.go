package xrpverifier

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	nodetypes "github.com/flare-foundation/tee-node/pkg/types"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"

	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var ErrValidationFailed = errors.New("multisig account validation failed")

type XRPVerifier struct {
	Config *config.PMWMultisigAccountConfig
	Client *client.Client
}

func NewXRPVerifier(cfg *config.PMWMultisigAccountConfig) *XRPVerifier {
	client := client.NewClient(cfg.RPCURL)

	return &XRPVerifier{Config: cfg, Client: client}
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	accountInfo, err := x.Client.GetAccountInfo(ctx, req.AccountAddress)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	sequence, err := x.validateMultisigConfiguration(accountInfo, req)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(apitypes.PMWMultisigAccountStatusERROR),
			Sequence: 0,
		}, nil
	}
	return connector.IPMWMultisigAccountConfiguredResponseBody{
		Status:   uint8(apitypes.PMWMultisigAccountStatusOK),
		Sequence: sequence,
	}, nil
}

func (x *XRPVerifier) validateMultisigConfiguration(accountInfo *types.AccountInfoResponse, req connector.IPMWMultisigAccountConfiguredRequestBody) (uint64, error) {
	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	if len(accountInfo.Result.AccountData.SignerLists) == 0 {
		return 0, fmt.Errorf("no signer list for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	signersValid := x.validateSignerList(accountInfo.Result.AccountData.SignerLists[0], req)
	if !signersValid {
		return 0, fmt.Errorf("signer list invalid for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	flags := accountInfo.Result.AccountFlags
	if err := checkAccountFlags(flags); err != nil {
		return 0, fmt.Errorf("invalid flag for account%s: %w: %w", accountInfo.Result.AccountData.Account, err, ErrValidationFailed)
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		return 0, fmt.Errorf("account %s has regular key set: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	return accountInfo.Result.AccountData.Sequence, nil
}

func (x *XRPVerifier) validateSignerList(signerList types.SignerList, req connector.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make([]string, len(req.PublicKeys))
	for i, pk := range req.PublicKeys {
		addrStr, err := XRPAddressFromPubKey(pk)
		if err != nil {
			logger.Warnf("Failed to convert public key %s to address: %v", hex.EncodeToString(pk), err)
			return false
		}
		expectedAccounts[i] = addrStr
	}
	actualAccounts := signerList.AccountsMap()
	if len(actualAccounts) != len(expectedAccounts) {
		return false
	}
	for _, acc := range expectedAccounts {
		weight, found := actualAccounts[acc]
		if !found || weight != 1 {
			return false
		}
	}
	return signerList.SignerQuorum == req.Threshold
}

func XRPAddressFromPubKey(pubkey []byte) (string, error) {
	pk, err := nodetypes.ParsePubKeyBytes(pubkey)
	if err != nil {
		return "", err
	}
	compressed := secp256k1.CompressPubkey(pk.X, pk.Y)
	return address.PubToAddress(hex.EncodeToString(compressed))
}

func checkAccountFlags(flags types.AccountFlags) error {
	switch {
	case !flags.DisableMasterKey:
		return errors.New("master key is not disabled")
	case flags.DepositAuth:
		return errors.New("deposit authorization is enabled")
	case flags.RequireDestinationTag:
		return errors.New("destination tag is required")
	case flags.DisallowIncomingXRP:
		return errors.New("incoming XRP is disallowed")
	}
	return nil
}
