package verifier

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
	xrpClient "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/verifier/xrp"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
	client *xrpClient.Client
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
		logger.Debugf("Account validation failed (Failed to get account info): %v", err)
		return 0, false, err
	}

	// There is only a single signer list for an account.
	// From docs: If a future amendment allows multiple signer lists for an account, this may change.[https://xrpl.org/docs/references/protocol/ledger-data/ledger-entry-types/signerlist]
	if len(accountInfo.Result.AccountData.SignerLists) == 0 {
		logger.Debug("Account validation failed: doesn't have signers")
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

func (x *XRPVerifier) verifySignerList(signerList xrpClient.SignerList, req connector.IPMWMultisigAccountConfiguredRequestBody) bool {
	expectedAccounts := make([]string, 0, len(req.PublicKeys))
	for _, pk := range req.PublicKeys {
		addrStr, err := convertPubkeyToAddress(pk)
		if err != nil {
			logger.Debugf("Account validation failed (Failed to convert public key to address): %v", err)
			return false
		}
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

func convertPubkeyToAddress(pubkey []byte) (string, error) {
	const pubKeyLength = 64
	if len(pubkey) != pubKeyLength {
		return "", errors.New("invalid public key length")
	}
	pk, err := ParsePubKey([64]byte(pubkey))
	if err != nil {
		return "", err
	}
	compressed := secp256k1.CompressPubkey(pk.X, pk.Y)
	return address.PubToAddress(hex.EncodeToString(compressed))
}

func ParsePubKey(pubkey [64]byte) (*ecdsa.PublicKey, error) {
	x := new(big.Int).SetBytes(pubkey[:32])
	y := new(big.Int).SetBytes(pubkey[32:])
	check := secp256k1.S256().IsOnCurve(x, y)
	if !check {
		return nil, errors.New("invalid public key bytes")
	}
	return &ecdsa.PublicKey{Curve: secp256k1.S256(), X: x, Y: y}, nil
}
