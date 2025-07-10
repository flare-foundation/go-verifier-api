package teeavailabilitycheckconfig

import (
	"fmt"
	"os"
)

type TeeAvailabilityCheckConfig struct {
	SourceID                   string
	RelayContractAddress       string
	TeeRegistryContractAddress string
	RPCURL                     string
}

func LoadTeeAvailabilityCheckConfig() (*TeeAvailabilityCheckConfig, error) {
	sourceID := os.Getenv("SOURCE_ID")
	if sourceID == "" {
		return nil, fmt.Errorf("SOURCE_ID not set")
	}
	if len(sourceID) > 32 {
		return nil, fmt.Errorf("SOURCE_ID longer than 32 bytes")
	}
	relayContractAddress := os.Getenv("RELAY_CONTRACT_ADDRESS")
	if relayContractAddress == "" {
		return nil, fmt.Errorf("RELAY_CONTRACT_ADDRESS not set")
	}
	teeRegistryContractAddress := os.Getenv("TEE_REGISTRY_CONTRACT_ADDRESS")
	if teeRegistryContractAddress == "" {
		return nil, fmt.Errorf("TEE_REGISTRY_CONTRACT_ADDRESS not set")
	}
	rpcURL := os.Getenv("RPC_URL")
	if teeRegistryContractAddress == "" {
		return nil, fmt.Errorf("RPC_URL not set")
	}

	return &TeeAvailabilityCheckConfig{
		SourceID:                   sourceID,
		RelayContractAddress:       relayContractAddress,
		TeeRegistryContractAddress: teeRegistryContractAddress,
		RPCURL:                     rpcURL,
	}, nil
}
