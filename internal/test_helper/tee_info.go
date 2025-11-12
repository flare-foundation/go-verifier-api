package testhelper

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
)

func GetTeeInfoResponse(chainChallenge common.Hash, privateKey *ecdsa.PrivateKey, teeTimestamp uint64) teenodetypes.TeeInfoResponse {
	const initPolicy = 4819
	const lastPolicy = 4830
	teeInfoResponse := teenodetypes.TeeInfoResponse{
		TeeInfo: teenodetypes.TeeInfo{
			Challenge:                chainChallenge,
			InitialSigningPolicyID:   initPolicy,
			InitialSigningPolicyHash: common.HexToHash("0x78042a0613055ef7112c2385946ff2eef3d83bad9d67b5e2d825f0b30fa8aef3"),
			LastSigningPolicyID:      lastPolicy,
			LastSigningPolicyHash:    common.HexToHash("0x0102ae123095bc60c947ce0dd6f2e8ffcc757fa60e7e98f430f8fded9212cc6f"),
			TeeTimestamp:             teeTimestamp,
			PublicKey:                teenodetypes.PublicKey{X: common.Hash(privateKey.X.Bytes()), Y: common.Hash(privateKey.Y.Bytes())},
		},
	}
	return teeInfoResponse
}
