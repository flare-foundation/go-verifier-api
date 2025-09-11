package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var (
	testWalletAddress = "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	testPublicKeys    = [][]byte{{0x01, 0x02}}
	testThreshold     = uint64(2)
)

func TestValidateRequest(t *testing.T) {
	encodedAndAbi := loadEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)

	t.Run("Valid encodedReq", func(t *testing.T) {
		attBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, testWalletAddress, testPublicKeys, testThreshold)
		req := testhelper.CreateAttestationRequest(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBody)
		err := ValidateRequest(req, encodedAndAbi)
		require.NoError(t, err)
	})

	t.Run("Invalid attestation type/source id", func(t *testing.T) {
		attBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, testWalletAddress, testPublicKeys, testThreshold)
		invalidReq := testhelper.CreateAttestationRequest(t, [32]byte{0xFF}, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBody)
		err := ValidateRequest(invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func TestValidateAndPrepareRequestBody(t *testing.T) {
	encodedAndAbi := loadEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	hexKeys := make([]hexutil.Bytes, len(testPublicKeys))
	for i, k := range testPublicKeys {
		hexKeys[i] = hexutil.Bytes(k)
	}
	attBody := attestationtypes.PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testWalletAddress,
		PublicKeys:     hexKeys,
		Threshold:      testThreshold,
	}
	t.Run("Valid encodedReq", func(t *testing.T) {
		req := testhelper.CreateAttestationRequestData(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBody)
		_, err := ValidateAndPrepareRequestBody(req, encodedAndAbi)
		require.NoError(t, err)
	})

	t.Run("Invalid encodedReq - validation fails", func(t *testing.T) {
		attBodyCopy := attBody
		attBodyCopy.PublicKeys = append(attBodyCopy.PublicKeys, hexutil.Bytes{})
		invalidReq := testhelper.CreateAttestationRequestData(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBodyCopy)
		_, err := ValidateAndPrepareRequestBody(invalidReq, encodedAndAbi)
		fmt.Println(err)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Invalid attestation type/source id", func(t *testing.T) {
		invalidReq := testhelper.CreateAttestationRequestData(t, [32]byte{0xFF}, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBody)
		_, err := ValidateAndPrepareRequestBody(invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Invalid ABI encode", func(t *testing.T) {
		req := testhelper.CreateAttestationRequestData(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIDPair.SourceIDEncoded, attBody)
		encodedAndAbiCopy := encodedAndAbi
		encodedAndAbiCopy.ABIPair.Request = abi.Argument{}
		_, err := ValidateAndPrepareRequestBody(req, encodedAndAbi)
		fmt.Println(err)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func TestValidateRequestData(t *testing.T) {
	encodedAndAbi := loadEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)

	t.Run("Valid", func(t *testing.T) {
		validReq := testhelper.CreateAttestationRequestData(t,
			encodedAndAbi.AttestationTypePair.AttestationTypeEncoded,
			encodedAndAbi.SourceIDPair.SourceIDEncoded,
			attestationtypes.PMWMultisigAccountConfiguredRequestBody{
				AccountAddress: testWalletAddress,
				PublicKeys:     []hexutil.Bytes{testPublicKeys[0]},
				Threshold:      testThreshold,
			},
		)
		err := ValidateRequestData(validReq, encodedAndAbi)
		require.NoError(t, err)
	})

	t.Run("Validation error", func(t *testing.T) {
		invalidReq := testhelper.CreateAttestationRequestData(t,
			encodedAndAbi.AttestationTypePair.AttestationTypeEncoded,
			encodedAndAbi.SourceIDPair.SourceIDEncoded,
			attestationtypes.PMWMultisigAccountConfiguredRequestBody{},
		)
		err := ValidateRequestData(invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Attestation/source mismatch", func(t *testing.T) {
		mismatchReq := testhelper.CreateAttestationRequestData(t,
			[32]byte{0xFF},
			encodedAndAbi.SourceIDPair.SourceIDEncoded,
			attestationtypes.PMWMultisigAccountConfiguredRequestBody{
				AccountAddress: testWalletAddress,
				PublicKeys:     []hexutil.Bytes{testPublicKeys[0]},
				Threshold:      testThreshold,
			},
		)
		err := ValidateRequestData(mismatchReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func loadEncodedAndABI(t *testing.T, attestationType connector.AttestationType, sourceId config.SourceName) *config.EncodedAndABI {
	t.Helper()
	encodedAndAbi, err := config.LoadEncodedAndABI(config.EnvConfig{
		APIKeys:         nil,
		AttestationType: attestationType,
		SourceID:        sourceId,
	})
	require.NoError(t, err)
	return &encodedAndAbi
}

func assertHumaError(t *testing.T, err error, expectedStatus int) {
	t.Helper()
	var herr *huma.ErrorModel
	require.Error(t, err)
	require.ErrorAs(t, err, &herr)
	require.Equal(t, expectedStatus, herr.Status)
}
