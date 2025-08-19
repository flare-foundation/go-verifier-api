package pmwmultisigaccountconfig

import (
	"fmt"
	"os"
	"sync"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmwMultisigAccountConfig     *config.PMWMultisigAccountConfig
	pmwMultisigAccountConfigOnce sync.Once
	pmwMultisigAccountConfigErr  error
)

func GetPMWMultisigAccountConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.PMWMultisigAccountConfig, error) {
	pmwMultisigAccountConfigOnce.Do(func() {
		pmwMultisigAccountConfig, pmwMultisigAccountConfigErr = LoadPMWMultisigAccountConfig(sourceId, attestationType)
	})
	return pmwMultisigAccountConfig, pmwMultisigAccountConfigErr
}

func LoadPMWMultisigAccountConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.PMWMultisigAccountConfig, error) {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC_URL not set in .env")
	}
	commonConfig, err := config.LoadEncodedAndAbi(sourceId, attestationType)
	if err != nil {
		return nil, err
	}
	return &config.PMWMultisigAccountConfig{
		SourcePair:          commonConfig.SourceIdPair,
		RPCURL:              rpcURL,
		AttestationTypePair: commonConfig.AttestationTypePair,
		AbiPair:             commonConfig.AbiPair,
	}, nil
}
