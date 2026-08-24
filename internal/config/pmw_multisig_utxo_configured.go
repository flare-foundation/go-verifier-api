package config

import (
	"sync"
)

var (
	pmwMultisigUtxoConfig     *PMWMultisigUtxoConfig
	pmwMultisigUtxoConfigOnce sync.Once
	errPmwMultisigUtxoConfig  error
)

func LoadPMWMultisigUtxoConfiguredConfig(envConfig EnvConfig) (*PMWMultisigUtxoConfig, error) {
	pmwMultisigUtxoConfigOnce.Do(func() {
		pmwMultisigUtxoConfig, errPmwMultisigUtxoConfig = BuildPMWMultisigUtxoConfiguredConfig(envConfig)
	})
	return pmwMultisigUtxoConfig, errPmwMultisigUtxoConfig
}

func BuildPMWMultisigUtxoConfiguredConfig(envConfig EnvConfig) (*PMWMultisigUtxoConfig, error) {
	err := CheckMissingFields(envConfig, []string{EnvSourceRPCURL})
	if err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	return &PMWMultisigUtxoConfig{
		EncodedAndABI: commonConfig,
		SourceRPCURL:  envConfig.SourceRPCURL,
		BtcNetwork:    envConfig.BtcNetwork,
	}, nil
}

func ClearPMWMultisigUtxoConfiguredConfigForTest() {
	pmwMultisigUtxoConfig = nil
	pmwMultisigUtxoConfigOnce = sync.Once{}
	errPmwMultisigUtxoConfig = nil
}
