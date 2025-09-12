package xrpverifier

import (
	"crypto/ecdsa"
	"crypto/rand"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/xrp/type"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sequence uint64 = 42

func TestVerifyMultisigConfiguration(t *testing.T) {
	const testAccountName = "rTestAccount"
	verifier := &XRPVerifier{}
	testAccounts := createTestAccounts(3, t)

	t.Run("Success", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigSuccess(t, seq, err)
	})

	t.Run("Wrong signer weights", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 2}, 2)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("Wrong threshold", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			3,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("Missing public key", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{{}, testAccounts[1].PubKey},
			2,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 2)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("SignerList mismatch", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[2].PubKey},
			1,
		)
		signerList := makeSignerList([]string{"acc2"}, []uint16{1}, 1)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("SignerList missing signer", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address}, []uint16{1}, 1)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("MasterKey enabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(false, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("DepositAuth enabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(true, true, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("RequireDestinationTagEnabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(true, false, true, false)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("DisallowIncomingXRPEnabled", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(true, false, false, true)
		accountInfo := makeAccountInfo(signerList, flags, "", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})

	t.Run("RegularKeySet", func(t *testing.T) {
		req := makeIPMWMultisigAccountConfiguredRequestBody(
			testAccountName,
			[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey},
			1,
		)
		signerList := makeSignerList([]string{testAccounts[0].Address, testAccounts[1].Address}, []uint16{1, 1}, 1)
		flags := accountFlags(true, false, false, false)
		accountInfo := makeAccountInfo(signerList, flags, "somekey", sequence)
		seq, err := verifier.validateMultisigConfiguration(accountInfo, req)
		requireMultisigConfigFailed(t, seq, err)
	})
}

func TestParsePubKey(t *testing.T) {
	t.Run("ValidPubKey", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		assert.NoError(t, err)

		pubkey := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)

		parsed, err := parsePubKey([64]byte(pubkey))
		assert.NoError(t, err)
		assert.Equal(t, privKey.PublicKey.X, parsed.X)
		assert.Equal(t, privKey.PublicKey.Y, parsed.Y)
		assert.Equal(t, privKey.PublicKey.Curve, parsed.Curve)
	})

	t.Run("InvalidPubKey", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		assert.NoError(t, err)

		// Corrupt the public key
		pubkey := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
		pubkey[10] ^= 0xFF

		parsed, err := parsePubKey([64]byte(pubkey))
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})
}

func TestConvertPubkeyToAddress(t *testing.T) {
	t.Run("ValidPubKey", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		assert.NoError(t, err)

		pubkey := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
		addr, err := XRPAddressFromPubKey(pubkey)
		assert.NoError(t, err)
		assert.NotEmpty(t, addr)
	})

	t.Run("ParsePubKeyFails", func(t *testing.T) {
		privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		assert.NoError(t, err)

		// Corrupt the public key
		pubkey := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
		pubkey[5] ^= 0xFF

		addr, err := XRPAddressFromPubKey(pubkey)
		assert.Error(t, err)
		assert.Empty(t, addr)
	})

	t.Run("WrongPubKeyLength", func(t *testing.T) {
		pubkey := make([]byte, 63) // invalid length
		addr, err := XRPAddressFromPubKey(pubkey)
		assert.Error(t, err)
		assert.Empty(t, addr)
	})
}

func requireMultisigConfigFailed(t *testing.T, seq uint64, err error) {
	assert.Error(t, err)
	assert.Equal(t, uint64(0), seq)
}

func requireMultisigConfigSuccess(t *testing.T, seq uint64, err error) {
	assert.NoError(t, err)
	assert.Equal(t, seq, sequence)
}

func accountFlags(disableMasterKey bool, depositAuth bool, requireDestinationTag bool, disallowIncomingXRP bool) types.AccountFlags {
	return types.AccountFlags{
		DisableMasterKey:      disableMasterKey,
		DepositAuth:           depositAuth,
		RequireDestinationTag: requireDestinationTag,
		DisallowIncomingXRP:   disallowIncomingXRP,
	}
}

func makeSignerList(accounts []string, weights []uint16, quorum uint64) []types.SignerList {
	entries := make([]types.SignerEntryWrapper, len(accounts))
	for i, acc := range accounts {
		entries[i] = types.SignerEntryWrapper{
			SignerEntry: types.SignerEntry{
				Account:      acc,
				SignerWeight: weights[i],
			},
		}
	}
	return []types.SignerList{
		{
			SignerQuorum:  quorum,
			SignerEntries: entries,
		},
	}
}

func makeAccountInfo(signerLists []types.SignerList, flags types.AccountFlags, regularKey string, sequence uint64,
) *types.AccountInfoResponse {
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

func makeIPMWMultisigAccountConfiguredRequestBody(accountAddress string, publicKeys [][]byte, threshold uint64) connector.IPMWMultisigAccountConfiguredRequestBody {
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

func createTestAccounts(n int, t *testing.T) []testAccount {
	accounts := make([]testAccount, n)
	for i := 0; i < n; i++ {
		priv, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		assert.NoError(t, err)
		pubkey := append(priv.PublicKey.X.Bytes(), priv.PublicKey.Y.Bytes()...)
		address, err := XRPAddressFromPubKey(pubkey)
		require.NoError(t, err)
		accounts[i] = testAccount{
			Address: address,
			PubKey:  pubkey,
		}
	}
	return accounts
}
