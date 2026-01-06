package xrpverifier

import (
	"crypto/ecdsa"
	"crypto/rand"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/types"
	"github.com/stretchr/testify/require"
)

const sequence uint64 = 42

func TestVerifyMultisigConfiguration(t *testing.T) {
	const testAccountName = "rTestAccount"
	verifier := &XRPVerifier{}
	testAccounts := createTestAccounts(t, 3)

	t.Run("success", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		require.NoError(t, err)
		require.Equal(t, seq, sequence)
	})
	t.Run("wrong signer weights", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 2}, 2)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "signer list invalid for account")
	})
	t.Run("wrong threshold", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			3,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "signer list invalid for account")
	})
	t.Run("missing public key", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{{}, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "signer list invalid for account")
	})
	t.Run("signer list mismatch", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[2].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{"acc2"}, []uint16{1}, 1)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "signer list invalid for account")
	})
	t.Run("signer list missing signer", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{}, []uint16{}, 1)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "o signer list for account")
	})
	t.Run("MasterKey enabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(t, false, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "master key is not disabled")
	})
	t.Run("DepositAuth enabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(t, true, true, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "deposit authorization is enabled")
	})
	t.Run("RequireDestinationTagEnabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(t, true, false, true, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "destination tag is required")
	})
	t.Run("DisallowIncomingXRPEnabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(t, true, false, false, true)
		accountInfo := makeAccountInfo(t, signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "incoming XRP is disallowed")
	})
	t.Run("RegularKeySet", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			t,
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList(t, []string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(t, true, false, false, false)
		accountInfo := makeAccountInfo(t, signerList, flags, "somekey", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err, "has regular key set")
	})
}

func TestConvertPubkeyToAddress(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		require.NoError(t, err)

		pubkey := append(privKey.X.Bytes(), privKey.Y.Bytes()...)
		addr, err := XRPAddressFromPubKey(pubkey)
		require.NoError(t, err)
		require.NotEmpty(t, addr)
	})
	t.Run("corrupted", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		require.NoError(t, err)

		// Corrupt the public key
		pubkey := append(privKey.X.Bytes(), privKey.Y.Bytes()...)
		pubkey[5] ^= 0xFF

		addr, err := XRPAddressFromPubKey(pubkey)
		require.ErrorContains(t, err, "coordinates not on curve")
		require.Empty(t, addr)
	})
	t.Run("wrong length", func(t *testing.T) {
		pubkey := make([]byte, 63) // invalid length
		addr, err := XRPAddressFromPubKey(pubkey)
		require.ErrorContains(t, err, "invalid public key should be 64 bytes long")
		require.Empty(t, addr)
	})
}

func requireMultisigConfigFailed(t *testing.T, seq uint64, err error, errorMessage string) {
	t.Helper()
	require.ErrorContains(t, err, errorMessage)
	require.Equal(t, uint64(0), seq)
}

func accountFlags(t *testing.T, disableMasterKey bool, depositAuth bool, requireDestinationTag bool, disallowIncomingXRP bool) types.AccountFlags {
	t.Helper()
	return types.AccountFlags{
		DisableMasterKey:      disableMasterKey,
		DepositAuth:           depositAuth,
		RequireDestinationTag: requireDestinationTag,
		DisallowIncomingXRP:   disallowIncomingXRP,
	}
}

func makeSignerList(t *testing.T, accounts []string, weights []uint16, quorum uint64) []types.SignerList {
	t.Helper()
	entries := make([]types.SignerEntryWrapper, len(accounts))
	for i, acc := range accounts {
		entries[i] = types.SignerEntryWrapper{
			SignerEntry: types.SignerEntry{
				Account:      acc,
				SignerWeight: weights[i],
			},
		}
	}
	if len(accounts) == 0 {
		return []types.SignerList{}
	}
	return []types.SignerList{
		{
			SignerQuorum:  quorum,
			SignerEntries: entries,
		},
	}
}

func makeAccountInfo(t *testing.T, signerLists []types.SignerList, flags types.AccountFlags, regularKey string, sequence uint64,
) *types.AccountInfoResponse {
	t.Helper()
	return &types.AccountInfoResponse{
		Result: types.AccountInfoResult{
			AccountData: types.AccountData{
				Account:     "rTestAccount",
				Sequence:    sequence,
				RegularKey:  regularKey,
				SignerLists: signerLists,
			},
			AccountFlags: flags,
			Status:       "success",
		},
	}
}

func makeIPMWMultisigAccountConfiguredRequestBody(t *testing.T, accountAddress string, publicKeys [][]byte, threshold uint64) connector.IPMWMultisigAccountConfiguredRequestBody {
	t.Helper()
	return connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: accountAddress,
		PublicKeys:     publicKeys,
		Threshold:      threshold,
	}
}

type testAccount struct {
	Address string
	PubKey  []byte
}

func createTestAccounts(t *testing.T, n int) []testAccount {
	t.Helper()
	accounts := make([]testAccount, n)
	for i := 0; i < n; i++ {
		priv, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		require.NoError(t, err)
		pubkey := append(priv.X.Bytes(), priv.Y.Bytes()...)
		address, err := XRPAddressFromPubKey(pubkey)
		require.NoError(t, err)
		accounts[i] = testAccount{
			Address: address,
			PubKey:  pubkey,
		}
	}
	return accounts
}
