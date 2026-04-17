package feeproofservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// Docker-dependent test: Requires Docker services.
// See README.md, section "Running specific tests manually" for details.
func TestNewFeeProofService(t *testing.T) {
	t.Run("missing fields in env config", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL: "",
			CChainDatabaseURL: "",
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load PMWFeeProof config: missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL, TEE_INSTRUCTIONS_CONTRACT_ADDRESS")
		require.Nil(t, service)
	})
	t.Run("using unsupported source ID", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "UNSUPPORTED_SOURCE",
			AttestationType:                connector.PMWFeeProof,
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot initialize PMWFeeProof verifier: no verifier for sourceID: UNSUPPORTED_SOURCE")
		require.Nil(t, service)
	})
	t.Run("misconfigured Source DB", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres:",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "testXRP",
			AttestationType:                connector.PMWFeeProof,
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot connect to Source DB:")
		require.Nil(t, service)
	})
	t.Run("misconfigured CChain DB", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL:              "root:root@tcp()",
			TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "testXRP",
			AttestationType:                connector.PMWFeeProof,
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot connect to CChain DB:")
		require.Nil(t, service)
	})
	config.ClearPMWFeeProofConfigForTest()
}
