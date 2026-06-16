package xrpverifier

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	nodetypes "github.com/flare-foundation/tee-node/pkg/types"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"

	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	ErrValidationFailed = errors.New("multisig account validation failed")
	// ErrInvalidRequest is returned when the request shape violates documented
	// constraints (e.g. too many public keys, empty key entries). Maps to HTTP 400.
	ErrInvalidRequest = errors.New("invalid multisig request")
)

type XRPVerifier struct {
	Config *config.PMWMultisigAccountConfig
	Client *client.Client
}

func NewXRPVerifier(cfg *config.PMWMultisigAccountConfig) *XRPVerifier {
	client := client.NewClient(cfg.RPCURL)

	return &XRPVerifier{Config: cfg, Client: client}
}

func (x *XRPVerifier) Verify(ctx context.Context, req fdc2.IPMWMultisigAccountConfiguredRequestBody) (fdc2.IPMWMultisigAccountConfiguredResponseBody, error) {
	// Enforce request shape here so direct ABI callers (verify / prepareResponseBody)
	// cannot bypass the limits applied by the JSON ToInternal path.
	if err := apitypes.ValidatePublicKeys(req.PublicKeys); err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	accountInfo, err := x.Client.FetchAccountInfo(ctx, req.AccountAddress)
	if err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	sequence, err := x.validateMultisigConfiguration(accountInfo, req)
	if err != nil {
		return fdc2.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(apitypes.PMWMultisigAccountStatusERROR),
			Sequence: 0,
		}, nil
	}
	return fdc2.IPMWMultisigAccountConfiguredResponseBody{
		Status:   uint8(apitypes.PMWMultisigAccountStatusOK),
		Sequence: sequence,
	}, nil
}

func (x *XRPVerifier) validateMultisigConfiguration(accountInfo *types.AccountInfoResponse, req fdc2.IPMWMultisigAccountConfiguredRequestBody) (uint64, error) {
	// Bind the response back to the requested account before trusting any of its
	// fields: a misbehaving/compromised RPC must not be able to answer with a
	// different (correctly-configured) account's data and have it accepted for
	// req.AccountAddress. XRPL returns the classic address in account_data.Account;
	// normalize the request (which may be an X-address) to classic before comparing.
	reqAddr, err := helper.NormalizeAddress(req.AccountAddress)
	if err != nil {
		return 0, fmt.Errorf("invalid request account address %q: %w", req.AccountAddress, ErrValidationFailed)
	}
	respAddr, err := helper.NormalizeAddress(accountInfo.Result.AccountData.Account)
	if err != nil {
		return 0, fmt.Errorf("invalid account_info account %q: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	if reqAddr != respAddr {
		return 0, fmt.Errorf("account_info returned account %q for requested %q: %w", accountInfo.Result.AccountData.Account, req.AccountAddress, ErrValidationFailed)
	}
	if accountInfo.Result.Validated == nil || !*accountInfo.Result.Validated {
		return 0, fmt.Errorf("account_info response is not from a validated ledger for %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	signerLists := accountInfo.Result.ResolveSignerLists()
	if len(signerLists) == 0 {
		return 0, fmt.Errorf("no signer list for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	signersValid := x.validateSignerList(signerLists[0], req)
	if !signersValid {
		return 0, fmt.Errorf("signer list invalid for account %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	flags := accountInfo.Result.AccountFlags
	if flags == nil {
		return 0, fmt.Errorf("account_flags missing from account_info response for %s: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	if err := checkAccountFlags(*flags); err != nil {
		return 0, fmt.Errorf("invalid flag for account%s: %w: %w", accountInfo.Result.AccountData.Account, err, ErrValidationFailed)
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		return 0, fmt.Errorf("account %s has regular key set: %w", accountInfo.Result.AccountData.Account, ErrValidationFailed)
	}
	return accountInfo.Result.AccountData.Sequence, nil
}

func (x *XRPVerifier) validateSignerList(signerList types.SignerList, req fdc2.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make(map[string]struct{}, len(req.PublicKeys))
	for _, pk := range req.PublicKeys {
		addrStr, err := XRPAddressFromPubKey(pk)
		if err != nil {
			logger.Warnf("Failed to convert public key %s to address: %v", hex.EncodeToString(pk), err)
			return false
		}
		expectedAccounts[addrStr] = struct{}{}
	}
	actualAccounts := signerList.AccountsMap()
	if len(actualAccounts) != len(expectedAccounts) {
		return false
	}
	for acc := range expectedAccounts {
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
	compressed := crypto.CompressPubkey(pk)
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
