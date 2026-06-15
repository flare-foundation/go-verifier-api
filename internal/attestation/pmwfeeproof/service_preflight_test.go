package feeproofservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestNewFeeProofServicePreflight covers the constructor checks that happen
// BEFORE any DB connection, so they run in the default (no-Docker) suite. Keeping
// the unsupported-source case here guards the preflight ordering: if the SOURCE_ID
// check ever moves back below InitSourceDB, this test fails without Docker.
func TestNewFeeProofServicePreflight(t *testing.T) {
	t.Run("missing fields in env config", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL: "",
			CChainDatabaseURL: "",
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load PMWFeeProof config: missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL, FLARE_TEE_MANAGER_CONTRACT_ADDRESS")
		require.Nil(t, service)
	})
	t.Run("using unsupported source ID", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		// Valid-looking DB URLs: the preflight must reject the source BEFORE any
		// connection is attempted, so this passes without Docker.
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "UNSUPPORTED_SOURCE",
			AttestationType:                fdc2.PMWFeeProof,
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, `unsupported SOURCE_ID "UNSUPPORTED_SOURCE" for PMWFeeProof`)
		require.Nil(t, service)
	})
	config.ClearPMWFeeProofConfigForTest()
}
