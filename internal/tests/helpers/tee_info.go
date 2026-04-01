package helpers

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TeeInfoResponse(t *testing.T, chainChallenge common.Hash) (teenodetypes.TeeInfoResponse, *ecdsa.PrivateKey) {
	t.Helper()
	privTEEKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	const initPolicy = 4819
	const lastPolicy = 4830
	const teeTimestamp = uint64(1762771322)
	teeInfoResponse := teenodetypes.TeeInfoResponse{
		TeeInfo: teenodetypes.TeeInfo{
			Challenge:                chainChallenge,
			InitialSigningPolicyID:   initPolicy,
			InitialSigningPolicyHash: common.HexToHash("0x78042a0613055ef7112c2385946ff2eef3d83bad9d67b5e2d825f0b30fa8aef3"),
			LastSigningPolicyID:      lastPolicy,
			LastSigningPolicyHash:    common.HexToHash("0x0102ae123095bc60c947ce0dd6f2e8ffcc757fa60e7e98f430f8fded9212cc6f"),
			TeeTimestamp:             teeTimestamp,
			PublicKey:                teenodetypes.PublicKey{X: common.BytesToHash(privTEEKey.X.Bytes()), Y: common.BytesToHash(privTEEKey.Y.Bytes())},
		},
	}

	return teeInfoResponse, privTEEKey
}
