package verification

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/relay"
	"gitlab.com/urskak/verifier-api/pkg/tee_availability_check/config"
)

const regOperationType = "REG"
const attestationType = "ATTESTATION_TYPE"

func GenerateChallengeInstructionId(teeId common.Address, challenge *big.Int) string {
	reg := common.BytesToHash([]byte(regOperationType))
	teeAttestation := common.BytesToHash([]byte(attestationType))
	teeIdHash := common.BytesToHash(teeId.Bytes())
	challengeHash := common.BytesToHash(challenge.Bytes())
	challengeInstructionId := crypto.Keccak256(reg[:], teeAttestation[:], teeIdHash[:], challengeHash[:])
	return hex.EncodeToString(challengeInstructionId)
}

func GetLastSigningPolicyHashFromChain(client *ethclient.Client, lastSigningPolicyId *big.Int) (common.Hash, error) {
	contractAddrStr, err := config.RelayContractAddress()
	if err != nil {
		return common.Hash{}, err
	}
	contractAddress := common.HexToAddress(contractAddrStr)
	relayCaller, err := relay.NewRelayCaller(contractAddress, client)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create contract caller: %w", err)
	}
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	lastSigningPolicyHash, err := relayCaller.ToSigningPolicyHash(callOpts, lastSigningPolicyId)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call ToSigningPolicyHash: %w", err)
	}
	return lastSigningPolicyHash, nil
}
