package coreutil

import "github.com/ethereum/go-ethereum/accounts/abi"

func AbiType(t string) abi.Type {
	ty, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return ty
}
