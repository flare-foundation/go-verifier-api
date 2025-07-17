package crypto

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
)

func abiArgumentsForRequestBody() (abi.Arguments, error) {
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

	return abi.Arguments{
		{Type: addressType},
		{Type: stringType},
		{Type: uint256Type},
	}, nil
}

func AbiEncodeRequestBody(body attestationtypes.ITeeAvailabilityCheckRequestBody) ([]byte, error) {
	arguments, err := abiArgumentsForRequestBody()
	if err != nil {
		return nil, err
	}

	addr := common.HexToAddress(body.TeeId)

	challengeInt := new(big.Int)
	if _, ok := challengeInt.SetString(body.Challenge, 10); !ok {
		return nil, fmt.Errorf("invalid numeric challenge string: %s", body.Challenge)
	}

	return arguments.Pack(addr, body.Url, challengeInt)
}

func AbiDecodeRequestBody(data []byte) (attestationtypes.ITeeAvailabilityCheckRequestBody, error) {
	arguments, err := abiArgumentsForRequestBody()
	if err != nil {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, err
	}

	values, err := arguments.Unpack(data)
	if err != nil {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, err
	}
	if len(values) != 3 {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, fmt.Errorf("unexpected argument count: got %d", len(values))
	}

	addr, ok := values[0].(common.Address)
	if !ok {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, fmt.Errorf("expected address, got %T", values[0])
	}
	url, ok := values[1].(string)
	if !ok {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, fmt.Errorf("expected string, got %T", values[1])
	}
	challenge, ok := values[2].(*big.Int)
	if !ok {
		return attestationtypes.ITeeAvailabilityCheckRequestBody{}, fmt.Errorf("expected *big.Int, got %T", values[2])
	}

	return attestationtypes.ITeeAvailabilityCheckRequestBody{
		TeeId:     addr.Hex(),
		Url:       url,
		Challenge: challenge.String(),
	}, nil
}
