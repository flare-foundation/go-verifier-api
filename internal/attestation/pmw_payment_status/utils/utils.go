package pmwpaymentutils

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teewalletmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teewalletprojectmanager"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func GenerateInstructionId(walletId, opType [32]byte, nonce uint64) (common.Hash, error) {
	PAY, err := utils.Bytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}
	var nonceByte common.Hash
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), 32))

	buf := new(bytes.Buffer)
	buf.Write(opType[:])
	buf.Write(PAY[:])
	buf.Write(walletId[:])
	buf.Write(nonceByte[:])
	instructionId := crypto.Keccak256Hash(buf.Bytes())
	return instructionId, nil
}

func GetStringField(m map[string]interface{}, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func GetStandardAddressHash(address string) string {
	hash := crypto.Keccak256([]byte(address))
	return fmt.Sprintf("0x%x", hash)
}

func GetWalletOpType(walletID [32]byte, walletCaller *teewalletmanager.TeeWalletManagerCaller, projectCaller *teewalletprojectmanager.TeeWalletProjectManagerCaller) ([32]byte, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	projectID, err := walletCaller.GetWalletProjectId(callOpts, walletID)
	if err != nil {
		return [32]byte{}, fmt.Errorf("GetWalletOpType: %w", err)
	}
	opType, err := projectCaller.GetOpType(callOpts, projectID)
	if err != nil {
		return [32]byte{}, fmt.Errorf("GetWalletOpType: %w", err)
	}
	return opType, nil
}
