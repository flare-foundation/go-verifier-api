package config

import (
	"fmt"
	"os"
	"sync"
)

var (
	sourceID                   string
	relayContractAddress       string
	teeRegistryContractAddress string
	once                       sync.Once
	initErr                    error
)

func loadEnv() {
	sourceID = os.Getenv("SOURCE_ID")
	relayContractAddress = os.Getenv("RELAY_CONTRACT_ADDRESS")
	teeRegistryContractAddress = os.Getenv("TEE_REGISTRY_CONTRACT_ADDRESS")

	if sourceID == "" {
		initErr = fmt.Errorf("SOURCE_ID not set")
		return
	}
	if len(sourceID) > 32 {
		initErr = fmt.Errorf("SOURCE_ID longer than 32 bytes")
		return
	}
	if relayContractAddress == "" {
		initErr = fmt.Errorf("RELAY_CONTRACT_ADDRESS not set")
		return
	}
	if teeRegistryContractAddress == "" {
		initErr = fmt.Errorf("TEE_REGISTRY_CONTRACT_ADDRESS not set")
		return
	}
}

func Init() error {
	once.Do(loadEnv)
	return initErr
}

func SourceID() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return sourceID, nil
}

func RelayContractAddress() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return relayContractAddress, nil
}

func TeeRegistryContractAddress() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return teeRegistryContractAddress, nil
}
