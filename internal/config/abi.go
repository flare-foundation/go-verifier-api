package config

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

func GetAbiArguments(structNeeded string) (abi.Argument, error) {
	parsedABI, err := abi.JSON(strings.NewReader(connector.ConnectorMetaData.ABI))
	if err != nil {
		return abi.Argument{}, fmt.Errorf("failed to parse ABI: %v", err)
	}

	method, ok := parsedABI.Methods[structNeeded]
	if !ok || len(method.Inputs) != 1 {
		return abi.Argument{}, fmt.Errorf("invalid method definition for %s", structNeeded)
	}

	return method.Inputs[0], nil
}
