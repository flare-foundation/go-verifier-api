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
	sourceIdEnc, err := config.EncodeAttestationOrSourceName(string(sourceId))
	if err != nil {
		return nil, err
	}
	attestationTypeEnc, err := config.EncodeAttestationOrSourceName(string(attestationType))
	if err != nil {
		return nil, err
	}
	requestAbi, err := config.GetAbiArguments("pmwMultisigAccountConfiguredRequestBodyStruct")
	if err != nil {
		return nil, err
	}
	responseAbi, err := config.GetAbiArguments("pmwMultisigAccountConfiguredResponseBodyStruct")
	if err != nil {
		return nil, err
	}
	return &config.PMWMultisigAccountConfig{
		SourcePair:          config.SourceIdEncodedPair{SourceId: sourceId, SourceIdEncoded: sourceIdEnc},
		RPCURL:              rpcURL,
		AttestationTypePair: config.AttestationTypeEncodedPair{AttestationType: attestationType, AttestationTypeEncoded: attestationTypeEnc},
		AbiPair:             config.AbiArgPair{Request: requestAbi, Response: responseAbi},
	}, nil
}
