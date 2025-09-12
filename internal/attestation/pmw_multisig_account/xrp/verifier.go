package xrpverifier

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/address"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/xrp/client"
	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/xrp/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

var ErrValidationFailed = errors.New("multisig account validation failed")

type XRPVerifier struct {
	Config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
	Client *client.Client
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	accountInfo, err := x.Client.GetAccountInfo(ctx, req.AccountAddress)
	if err != nil {
		logger.Debugf("Failed to get account info: %v", err)
		return connector.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(attestationtypes.PMWMultisigAccountStatusERROR),
			Sequence: 0,
		}, nil
	}

	sequence, err := x.validateMultisigConfiguration(accountInfo, req)
	if err != nil {
		if errors.Is(err, ErrValidationFailed) {
			return connector.IPMWMultisigAccountConfiguredResponseBody{
				Status:   uint8(attestationtypes.PMWMultisigAccountStatusERROR),
				Sequence: 0,
			}, nil
		}
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, err
	}
	return connector.IPMWMultisigAccountConfiguredResponseBody{
		Status:   uint8(attestationtypes.PMWMultisigAccountStatusOK),
		Sequence: sequence,
	}, nil

}

func (x *XRPVerifier) validateMultisigConfiguration(accountInfo *types.AccountInfoResponse, req connector.IPMWMultisigAccountConfiguredRequestBody) (uint64, error) {
	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	if len(accountInfo.Result.AccountData.SignerLists) == 0 {
		logger.Debug("Account has no signer list")
		return 0, ErrValidationFailed
	}
	signersValid := x.validateSignerList(accountInfo.Result.AccountData.SignerLists[0], req)
	if !signersValid {
		return 0, ErrValidationFailed
	}
	flags := accountInfo.Result.AccountFlags
	if err := checkAccountFlags(flags); err != nil {
		logger.Debugf("Invalid account flags: %v", err)
		return 0, ErrValidationFailed
	}
	if accountInfo.Result.AccountData.RegularKey != "" {
		logger.Debug("Account has regular key set")
		return 0, ErrValidationFailed
	}
	return accountInfo.Result.AccountData.Sequence, nil
}

func (x *XRPVerifier) validateSignerList(signerList types.SignerList, req connector.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make([]string, len(req.PublicKeys))
	for i, pk := range req.PublicKeys {
		addrStr, err := XRPAddressFromPubKey(pk)
		if err != nil {
			logger.Debugf("Failed to convert public key to address: %v", err)
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
	const pubKeyLength = 64
	if len(pubkey) != pubKeyLength {
		return "", errors.New("invalid public key length")
	}
	pk, err := parsePubKey([64]byte(pubkey))
	if err != nil {
		return "", err
	}
	compressed := secp256k1.CompressPubkey(pk.X, pk.Y)
	return address.PubToAddress(hex.EncodeToString(compressed))
}

func parsePubKey(pubkey [64]byte) (*ecdsa.PublicKey, error) {
	x := new(big.Int).SetBytes(pubkey[:32])
	y := new(big.Int).SetBytes(pubkey[32:])
	check := secp256k1.S256().IsOnCurve(x, y)
	if !check {
		return nil, errors.New("invalid public key bytes")
	}
	return &ecdsa.PublicKey{Curve: secp256k1.S256(), X: x, Y: y}, nil
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
