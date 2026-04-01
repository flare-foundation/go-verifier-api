package teeavailabilityservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
	RelayContractAddress:              "0x0000000000000000000000000000000000000001",
	TeeMachineRegistryContractAddress: "0x0000000000000000000000000000000000000002",
	SourceID:                          config.SourceTEE,
	AttestationType:                   connector.AvailabilityCheck,
}

func TestTeeAvailabilityService(t *testing.T) {
	t.Run("should successfully create TeeAvailabilityService", func(t *testing.T) {
		config.ClearTeeAvailabilityCheckConfigForTest()

		service, err := NewTeeAvailabilityService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.Verifier())
		require.NotNil(t, service.Config())
	})

	t.Run("missing fields in env config", func(t *testing.T) {
		config.ClearTeeAvailabilityCheckConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:                            "",
			RelayContractAddress:              envConfig.RelayContractAddress,
			TeeMachineRegistryContractAddress: envConfig.TeeMachineRegistryContractAddress,
			SourceID:                          envConfig.SourceID,
			AttestationType:                   envConfig.AttestationType,
		}
		service, err := NewTeeAvailabilityService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load TeeAvailabilityCheck config: missing environment variables: RPC_URL")
		require.Nil(t, service)
	})

	t.Run("unknown attestation type", func(t *testing.T) {
		config.ClearTeeAvailabilityCheckConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:                            envConfig.RPCURL,
			RelayContractAddress:              envConfig.RelayContractAddress,
			TeeMachineRegistryContractAddress: envConfig.TeeMachineRegistryContractAddress,
			SourceID:                          envConfig.SourceID,
			AttestationType:                   "UnknownType",
		}
		service, err := NewTeeAvailabilityService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load TeeAvailabilityCheck config: no ABI struct names defined for attestation type UnknownType")
		require.Nil(t, service)
	})

	config.ClearTeeAvailabilityCheckConfigForTest()
}
