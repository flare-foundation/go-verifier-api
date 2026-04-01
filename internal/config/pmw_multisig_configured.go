package config

import (
	"sync"
)

var (
	pmwMultisigAccountConfig     *PMWMultisigAccountConfig
	pmwMultisigAccountConfigOnce sync.Once
	errPmwMultisigAccountConfig  error
)

func LoadPMWMultisigAccountConfiguredConfig(envConfig EnvConfig) (*PMWMultisigAccountConfig, error) {
	pmwMultisigAccountConfigOnce.Do(func() {
		pmwMultisigAccountConfig, errPmwMultisigAccountConfig = BuildPMWMultisigAccountConfiguredConfig(envConfig)
	})
	return pmwMultisigAccountConfig, errPmwMultisigAccountConfig
}

func BuildPMWMultisigAccountConfiguredConfig(envConfig EnvConfig) (*PMWMultisigAccountConfig, error) {
	err := CheckMissingFields(envConfig, []string{EnvRPCURL})
	if err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	return &PMWMultisigAccountConfig{
		EncodedAndABI: commonConfig,
		RPCURL:        envConfig.RPCURL,
	}, nil
}

func ClearPMWMultisigAccountConfiguredConfigForTest() {
	pmwMultisigAccountConfig = nil
	pmwMultisigAccountConfigOnce = sync.Once{}
	errPmwMultisigAccountConfig = nil
}
