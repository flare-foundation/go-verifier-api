package keyutil_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/keyutil"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestRecoverSigner(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	t.Run("valid signature", func(t *testing.T) {
		expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey)
		data := []byte("hello world")
		hash := crypto.Keccak256(data)
		signature, err := crypto.Sign(accounts.TextHash(hash), privKey)
		require.NoError(t, err)

		addr, err := keyutil.RecoverSigner(data, signature)
		require.NoError(t, err)
		require.Equal(t, expectedAddr, addr)
	})
	t.Run("invalid signature", func(t *testing.T) {
		data := []byte("hello")
		invalidSig := []byte("notavalidsignature")

		addr, err := keyutil.RecoverSigner(data, invalidSig)
		require.ErrorContains(t, err, "failed to recover pubkey: invalid signature length")
		require.Equal(t, common.Address{}, addr)
	})
	t.Run("empty data", func(t *testing.T) {
		signature, err := crypto.Sign(accounts.TextHash(crypto.Keccak256([]byte{})), privKey)
		require.NoError(t, err)

		addr, err := keyutil.RecoverSigner([]byte{}, signature)
		require.NoError(t, err)
		require.Equal(t, crypto.PubkeyToAddress(privKey.PublicKey), addr)
	})
	t.Run("truncated signature", func(t *testing.T) {
		data := []byte("hello world")
		hash := crypto.Keccak256(data)
		signature, err := crypto.Sign(accounts.TextHash(hash), privKey)
		require.NoError(t, err)

		// Remove last byte to make it invalid
		truncatedSig := signature[:len(signature)-1]

		addr, err := keyutil.RecoverSigner(data, truncatedSig)
		require.ErrorContains(t, err, "failed to recover pubkey: invalid signature length")
		require.Equal(t, common.Address{}, addr)
	})
}

func TestRetrieveAddressFromPublicKey(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		pub := teenodetypes.PublicKey{
			X: common.Hash(key.X.Bytes()),
			Y: common.Hash(key.Y.Bytes()),
		}
		addr, err := keyutil.RetrieveAddressFromPublicKey(pub)
		require.NoError(t, err)
		require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), addr)
	})
	t.Run("invalid", func(t *testing.T) {
		pub := teenodetypes.PublicKey{
			X: common.BigToHash(big.NewInt(123)),
			Y: common.BigToHash(big.NewInt(456)),
		}
		val, err := keyutil.RetrieveAddressFromPublicKey(pub)
		require.ErrorContains(t, err, "invalid public key")
		require.Equal(t, common.Address{}, val)
	})
}
