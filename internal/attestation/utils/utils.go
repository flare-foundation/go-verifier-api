package utils

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
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
	arg, err := getTeeAvailabilityCheckRequestBodyStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckRequestBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckRequestBody': %v", err)
	}
	fmt.Println(arg)
	structs.Encode(connector.AttestationRequestArg, &connector.IFtdcHubFtdcAttestationRequest{})

	return encoded, nil
}

func AbiDecodeRequestData(data []byte) (types.TeeAvailabilityRequestData, error) {
	arg, err := getTeeAvailabilityCheckRequestBodyStruct()
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
	arg, err := getTeeAvailabilityCheckResponseBodyStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckResponseBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	fmt.Println(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckResponseBody': %v", err)
	}
	return encoded, nil
}

func getTeeAvailabilityCheckRequestBodyStruct() (abi.Argument, error) {
	parsedABI, err := abi.JSON(strings.NewReader(connector.ConnectorMetaData.ABI))
	if err != nil {
		return abi.Argument{}, fmt.Errorf("failed to parse ABI: %v", err)
	}
	method := parsedABI.Methods["availabilityCheckRequestBodyStruct"]
	if len(method.Inputs) != 1 {
		return abi.Argument{}, fmt.Errorf("expected 1 input in 'availabilityCheckRequestBodyStruct', got %d", len(method.Inputs))
	}
	return method.Inputs[0], nil
}

func getTeeAvailabilityCheckResponseBodyStruct() (abi.Argument, error) {
	parsedABI, err := abi.JSON(strings.NewReader(connector.ConnectorMetaData.ABI))
	if err != nil {
		return abi.Argument{}, fmt.Errorf("failed to parse ABI: %v", err)
	}
	method := parsedABI.Methods["availabilityCheckResponseBodyStruct"]
	if len(method.Inputs) != 1 {
		return abi.Argument{}, fmt.Errorf("expected 1 input in 'availabilityCheckResponseBodyStruct', got %d", len(method.Inputs))
	}
	return method.Inputs[0], nil
}
