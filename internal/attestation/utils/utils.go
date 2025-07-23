package utils

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	teeavailabilitycheckconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
)

func Bytes32(s string) [32]byte {
	var b [32]byte
	// if len(s) > 32 { // TODO - need this check?
	// 	return b, fmt.Errorf("string too long for Bytes32")
	// }
	copy(b[:], s)
	return b
}

func AbiEncodeRequestData(data types.TeeAvailabilityRequestData) ([]byte, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeRequestArg()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckRequestBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckRequestBody': %v", err)
	}
	structs.Encode(connector.AttestationRequestArg, &connector.IFtdcHubFtdcAttestationRequest{})

	return encoded, nil
}

func AbiDecodeRequestData(data []byte) (types.TeeAvailabilityRequestData, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeRequestArg()
	if err != nil {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("failed to get 'TeeAvailabilityCheckRequestBody' ABI argument: %v", err)
	}
	decode, err := structs.Decode[types.TeeAvailabilityRequestData](arg, data)
	if err != nil {
		return types.TeeAvailabilityRequestData{}, err
	}
	return decode, nil
}

func AbiEncodeResponseData(data types.TeeAvailabilityResponseData) ([]byte, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeResponseArg()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckResponseBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckResponseBody': %v", err)
	}
	return encoded, nil
}
