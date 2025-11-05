package coreutil

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func mustAbiType(t string) abi.Type {
	ty, err := abi.NewType(t, "", nil)
	if err != nil {
		panic("invalid ABI type: " + err.Error())
	}
	return ty
}

var (
	Bytes32Type = mustAbiType("bytes32")
	Uint64Type  = mustAbiType("uint64")
	StringType  = mustAbiType("string")
)
