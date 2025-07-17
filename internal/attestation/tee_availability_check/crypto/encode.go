package crypto

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/type"
)

func abiArgumentsForRequestData() (abi.Arguments, error) {
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

func AbiEncodeRequestBody(body attestationtypes.TeeAvailabilityRequestData) ([]byte, error) {
	arguments, err := abiArgumentsForRequestData()
	if err != nil {
		return nil, err
	}
	return arguments.Pack(body.TeeId, body.Url, body.Challenge)
}

func AbiDecodeRequestData(data []byte) (attestationtypes.TeeAvailabilityRequestData, error) {
	arguments, err := abiArgumentsForRequestData()
	if err != nil {
		return attestationtypes.TeeAvailabilityRequestData{}, err
	}

	values, err := arguments.Unpack(data)
	if err != nil {
		return attestationtypes.TeeAvailabilityRequestData{}, err
	}
	if len(values) != 3 {
		return attestationtypes.TeeAvailabilityRequestData{}, fmt.Errorf("unexpected argument count: got %d", len(values))
	}

	addr, ok := values[0].(common.Address)
	if !ok {
		return attestationtypes.TeeAvailabilityRequestData{}, fmt.Errorf("expected address, got %T", values[0])
	}
	url, ok := values[1].(string)
	if !ok {
		return attestationtypes.TeeAvailabilityRequestData{}, fmt.Errorf("expected string, got %T", values[1])
	}
	challenge, ok := values[2].(*big.Int)
	if !ok {
		return attestationtypes.TeeAvailabilityRequestData{}, fmt.Errorf("expected *big.Int, got %T", values[2])
	}

	return attestationtypes.TeeAvailabilityRequestData{
		TeeId:     addr,
		Url:       url,
		Challenge: challenge,
	}, nil
}
