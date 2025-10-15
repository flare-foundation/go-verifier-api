package api

import (
	"context"
	"net/http"
	"testing"

	teeavailabilityconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

type mockAdapter struct{}

func (mockAdapter) Handle(op *huma.Operation, handler func(ctx huma.Context)) {}
func (mockAdapter) ServeHTTP(http.ResponseWriter, *http.Request)              {}

func TestUnsupportedAttestationType(t *testing.T) {
	ctx := context.Background()
	api := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})

	envConfig := config.EnvConfig{
		AttestationType: "UnknownType",
	}
	closers, err := LoadModule(ctx, api, envConfig)
	require.ErrorContains(t, err, "unsupported attestation type")
	require.Nil(t, closers)
}

func TestTEEAvailabilityCheckRPCDialError(t *testing.T) {
	teeavailabilityconfig.ResetTeeAvailabilityCheckConfigForTest()
	api := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})

	envConfig := config.EnvConfig{
		AttestationType:                   connector.AvailabilityCheck,
		RPCURL:                            "http",
		RelayContractAddress:              "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		AllowTeeDebug:                     "false",
		DisableAttestationCheckE2E:        "false",
	}
	closers, err := LoadModule(t.Context(), api, envConfig)
	require.ErrorContains(t, err, "cannot connect to Flare node")
	require.Nil(t, closers)
}

func TestTEEAvailabilityCheckConfigError(t *testing.T) {
	teeavailabilityconfig.ResetTeeAvailabilityCheckConfigForTest()
	api := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})

	envConfig := config.EnvConfig{
		AttestationType: connector.AvailabilityCheck,
	}
	closers, err := LoadModule(t.Context(), api, envConfig)
	require.ErrorContains(t, err, "cannot retrieve config")
	require.Nil(t, closers)
}

func TestPMWPaymentStatusServiceError(t *testing.T) {
	api := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})

	envConfig := config.EnvConfig{
		AttestationType: connector.PMWPaymentStatus,
	}
	closers, err := LoadModule(t.Context(), api, envConfig)
	require.ErrorContains(t, err, "cannot load PMWPaymentStatus config: missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL")
	require.Nil(t, closers)
}

func TestPMWMultisigAccountConfiguredServiceError(t *testing.T) {
	api := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})

	envConfig := config.EnvConfig{
		AttestationType: connector.PMWMultisigAccountConfigured,
	}
	closers, err := LoadModule(t.Context(), api, envConfig)
	require.ErrorContains(t, err, "cannot load PMWMultisigAccountConfigured config: missing environment variables: RPC_URL")
	require.Nil(t, closers)
}
