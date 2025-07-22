package crypto

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
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
	bytesVal, ok := values[2].([32]byte)
	if !ok {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("expected [32]byte for common.Hash, got %T", values[2])
	}
	var challenge common.Hash
	copy(challenge[:], bytesVal[:])

	return types.TeeAvailabilityRequestData{
		TeeId:     addr,
		Url:       url,
		Challenge: challenge,
	}, nil
}
