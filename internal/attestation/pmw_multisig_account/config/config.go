package pmwmultisigaccountconfig

import (
	"fmt"
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
	if envConfig.RPCURL == "" {
		return nil, fmt.Errorf("RPC_URL not set in .env")
	}
	if envConfig.XRPClientURL == "" {
		return nil, fmt.Errorf("XRP_CLIENT_URL not set in .env")
	}
	commonConfig, err := config.LoadEncodedAndAbi(envConfig)
	if err != nil {
		return nil, err
	}
	return &config.PMWMultisigAccountConfig{
		SourcePair:          commonConfig.SourceIdPair,
		RPCURL:              envConfig.RPCURL,
		XRPClientURL:        envConfig.XRPClientURL,
		AttestationTypePair: commonConfig.AttestationTypePair,
		AbiPair:             commonConfig.AbiPair,
	}, nil
}
