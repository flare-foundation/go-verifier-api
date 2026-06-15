//go:build integration

package feeproofservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// Docker-dependent test: gated behind the `integration` build tag and requires the
// DB fixtures (see README.md, "Running Tests"). Run with `go test -tags integration`.
// Non-DB constructor cases (missing config, unsupported source) live in the untagged
// service_preflight_test.go so they run in the default suite.
func TestNewFeeProofService(t *testing.T) {
	t.Run("misconfigured Source DB", func(t *testing.T) {
		config.ClearPMWFeeProofConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres:",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "testXRP",
			AttestationType:                fdc2.PMWFeeProof,
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
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			SourceID:                       "testXRP",
			AttestationType:                fdc2.PMWFeeProof,
		}
		service, err := NewFeeProofService(badEnvConfig)
		require.ErrorContains(t, err, "cannot connect to CChain DB:")
		require.Nil(t, service)
	})
	config.ClearPMWFeeProofConfigForTest()
}
