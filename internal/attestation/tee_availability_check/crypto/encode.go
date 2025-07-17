package crypto

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
)

func AbiEncodeRequestBody(body attestationtypes.ITeeAvailabilityCheckRequestBody) ([]byte, error) {
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		return nil, err
	}
	stringType, err := abi.NewType("string", "", nil)
	if err != nil {
		return nil, err
	}
	uint256Type, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return nil, err
	}

	arguments := abi.Arguments{ //TODO
		{Type: addressType},
		{Type: stringType},
		{Type: uint256Type},
	}

	addr := common.HexToAddress(body.TeeId)

	challengeInt := new(big.Int)
	if _, ok := challengeInt.SetString(body.Challenge, 10); !ok {
		return nil, fmt.Errorf("invalid numeric challenge string: %s", body.Challenge)
	}

	return arguments.Pack(addr, body.Url, challengeInt)
}
