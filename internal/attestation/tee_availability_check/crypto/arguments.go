package crypto

import "github.com/ethereum/go-ethereum/accounts/abi"

func AbiArgumentsForRequestData() (abi.Arguments, error) {
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
		{Type: addressType}, //teeId
		{Type: stringType},  //url
		{Type: uint256Type}, //challenge
	}, nil
}

func AbiArgumentsForResponseData() (abi.Arguments, error) {
	uint8Type, err := abi.NewType("uint8", "", nil)
	if err != nil {
		return nil, err
	}
	uint64Type, err := abi.NewType("uint64", "", nil)
	if err != nil {
		return nil, err
	}
	uint24Type, err := abi.NewType("uint24", "", nil)
	if err != nil {
		return nil, err
	}
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return nil, err
	}

	return abi.Arguments{
		{Type: uint8Type},   //status
		{Type: uint64Type},  //teeTimestamp
		{Type: bytes32Type}, //codeHash
		{Type: bytes32Type}, //platform
		{Type: uint24Type},  //lastSigningPolicyId
		{Type: uint24Type},  //initialSigningPolicyId
		{Type: bytes32Type}, //stateHash
	}, nil
}
