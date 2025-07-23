package teeavailabilitycheckconfig

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

var (
	teeRequestArg  abi.Argument
	teeResponseArg abi.Argument
	abiInitErr     error
)

func init() {
	var parsedABI abi.ABI
	parsedABI, abiInitErr = abi.JSON(strings.NewReader(connector.ConnectorMetaData.ABI))
	if abiInitErr != nil {
		abiInitErr = fmt.Errorf("failed to parse ABI: %v", abiInitErr)
		return
	}

	reqMethod, ok := parsedABI.Methods["availabilityCheckRequestBodyStruct"]
	if !ok || len(reqMethod.Inputs) != 1 {
		abiInitErr = fmt.Errorf("invalid request method definition")
		return
	}
	teeRequestArg = reqMethod.Inputs[0]

	respMethod, ok := parsedABI.Methods["availabilityCheckResponseBodyStruct"]
	if !ok || len(respMethod.Inputs) != 1 {
		abiInitErr = fmt.Errorf("invalid response method definition")
		return
	}
	teeResponseArg = respMethod.Inputs[0]
}

func GetTeeRequestArg() (abi.Argument, error) {
	if abiInitErr != nil {
		return abi.Argument{}, abiInitErr
	}
	return teeRequestArg, nil
}

func GetTeeResponseArg() (abi.Argument, error) {
	if abiInitErr != nil {
		return abi.Argument{}, abiInitErr
	}
	return teeResponseArg, nil
}
