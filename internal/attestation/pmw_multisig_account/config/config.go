package pmwmultisigaccountconfig

import (
	"sync"

	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmwMultisigAccountConfig     *config.PMWMultisigAccountConfig
	pmwMultisigAccountConfigOnce sync.Once
	errPmwMultisigAccountConfig  error
)

func GetPMWMultisigAccountConfig(envConfig config.EnvConfig) (*config.PMWMultisigAccountConfig, error) {
	pmwMultisigAccountConfigOnce.Do(func() {
		pmwMultisigAccountConfig, errPmwMultisigAccountConfig = LoadPMWMultisigAccountConfig(envConfig)
	})
	return pmwMultisigAccountConfig, errPmwMultisigAccountConfig
}

func LoadPMWMultisigAccountConfig(envConfig config.EnvConfig) (*config.PMWMultisigAccountConfig, error) {
	err := config.CheckMissingFields(envConfig, []string{config.EnvRPCURL})
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndAbi(envConfig)
	if err != nil {
		return nil, err
	}
	return &config.PMWMultisigAccountConfig{
		EncodedAndAbi: commonConfig,
		RPCURL:        envConfig.RPCURL,
	}, nil
}

func ClearPMWMultisigAccountConfigForTest() {
	pmwMultisigAccountConfig = nil
	pmwMultisigAccountConfigOnce = sync.Once{}
	errPmwMultisigAccountConfig = nil
}
