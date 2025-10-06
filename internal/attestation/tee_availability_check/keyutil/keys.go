package keyutil

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
)

func RecoverSigner(data hexutil.Bytes, signature hexutil.Bytes) (common.Address, error) {
	hash := crypto.Keccak256(data)
	ethHash := accounts.TextHash(hash)
	pub, err := crypto.SigToPub(ethHash, signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot recover pubkey: %w", err)
	}
	return crypto.PubkeyToAddress(*pub), nil
}

func RetrieveAddressFromPublicKey(publicKey teenodetypes.PublicKey) (common.Address, error) {
	pubKey, err := teenodetypes.ParsePubKey(publicKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("invalid public key: %w", err)
	}
	return crypto.PubkeyToAddress(*pubKey), nil
}
