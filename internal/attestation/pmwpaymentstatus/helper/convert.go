package helper

import (
	"fmt"
	"math/big"
)

func ParseBigInt(s string) (*big.Int, error) {
	const decimalBase = 10
	i, ok := new(big.Int).SetString(s, decimalBase)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	return i, nil
}
