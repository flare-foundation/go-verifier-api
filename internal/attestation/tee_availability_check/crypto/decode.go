package crypto

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	types "gitlab.com/urskak/verifier-api/internal/api/type"
)

func AbiDecodeRequestData(data []byte) (types.TeeAvailabilityRequestData, error) {
	arguments, err := AbiArgumentsForRequestData()
	if err != nil {
		return types.TeeAvailabilityRequestData{}, err
	}

	values, err := arguments.Unpack(data)
	if err != nil {
		return types.TeeAvailabilityRequestData{}, err
	}
	if len(values) != 3 {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("unexpected argument count: got %d", len(values))
	}

	addr, ok := values[0].(common.Address)
	if !ok {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("expected address, got %T", values[0])
	}
	url, ok := values[1].(string)
	if !ok {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("expected string, got %T", values[1])
	}
	challenge, ok := values[2].(*big.Int)
	if !ok {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("expected *big.Int, got %T", values[2])
	}

	return types.TeeAvailabilityRequestData{
		TeeId:     addr,
		Url:       url,
		Challenge: challenge,
	}, nil
}
