//go:build integration

package server_test

import (
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/flare-foundation/go-verifier-api/internal/tests/server"
	"github.com/stretchr/testify/require"
)

// TestPerSourceMultiType starts a single server that serves every attestation
// type of one source (the per-source deployment shape) and asserts that each
// served type's routes are registered while an unserved type's are not.
//
// Docker-dependent test: requires the same services as the other integration
// tests. See README.md, section "Running specific tests manually".
func TestPerSourceMultiType(t *testing.T) {
	config.ClearPMWPaymentStatusConfigForTest()
	config.ClearPMWFeeProofConfigForTest()
	config.ClearPMWMultisigAccountConfiguredConfigForTest()

	rpc := server.MockEthRPC(t, 11263145)
	served := config.SourceAttestationTypes[config.SourceTestXRP]
	setup := server.SetupMultiServer(t, config.SourceTestXRP, served, config.EnvConfig{
		SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
		FlareTeeManagerContractAddress: "0x93c1e99c8dd990d77232821f9476c308fbad47f5",
		TeePaymentsContractAddress:     "0x93c1e99c8dd990d77232821f9476c308fbad47f5",
		// XRP deployment needs both RPCs: PaymentStatus/FeeProof dial Flare
		// (getInitialNonce); Multisig queries the XRP node.
		FlareRPCURL:  rpc.URL,
		SourceRPCURL: rpc.URL,
	})
	defer setup.Stop()

	// Every served type is registered: /verify without an API key hits the auth
	// middleware (401), proving the route exists rather than 404.
	for _, at := range served {
		t.Run("registered: "+string(at), func(t *testing.T) {
			resp, err := helpers.PostWithoutMarshalling(t, setup.URL(at)+"/verify", struct{}{}, "") //nolint:bodyclose // closed below
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "served type must be registered")
			_ = resp.Body.Close()
		})
	}

	// A type this source does not serve is not registered at all (404), proving
	// the deployment registers only its source's types.
	t.Run("unserved type is not registered", func(t *testing.T) {
		url := setup.URL(fdc2.AvailabilityCheck) + "/verify"
		resp, err := helpers.PostWithoutMarshalling(t, url, struct{}{}, setup.APIKey) //nolint:bodyclose // closed below
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode, "unserved type must not be registered")
		_ = resp.Body.Close()
	})
}
