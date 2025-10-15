package config

import (
	"sync"

	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmwMultisigAccountConfig     *config.PMWMultisigAccountConfig
	pmwMultisigAccountConfigOnce sync.Once
	errPmwMultisigAccountConfig  error
)

func GetPMWMultisigAccountConfiguredConfig(envConfig config.EnvConfig) (*config.PMWMultisigAccountConfig, error) {
	pmwMultisigAccountConfigOnce.Do(func() {
		pmwMultisigAccountConfig, errPmwMultisigAccountConfig = LoadPMWMultisigAccountConfiguredConfig(envConfig)
	})
	return pmwMultisigAccountConfig, errPmwMultisigAccountConfig
}

func LoadPMWMultisigAccountConfiguredConfig(envConfig config.EnvConfig) (*config.PMWMultisigAccountConfig, error) {
	err := config.CheckMissingFields(envConfig, []string{config.EnvRPCURL})
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	return &config.PMWMultisigAccountConfig{
		EncodedAndABI: commonConfig,
		RPCURL:        envConfig.RPCURL,
	}, nil
}

func ClearPMWMultisigAccountConfiguredConfigForTest() {
	pmwMultisigAccountConfig = nil
	pmwMultisigAccountConfigOnce = sync.Once{}
	errPmwMultisigAccountConfig = nil
}
