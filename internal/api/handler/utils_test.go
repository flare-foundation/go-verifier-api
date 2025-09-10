package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common/hexutil"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	testutil "github.com/flare-foundation/go-verifier-api/internal/test_util"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var (
	testWalletAddress = "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	testPublicKeys    = [][]byte{{0x01, 0x02}}
	testThreshold     = uint64(2)
	testSequence      = uint64(42)
	testStatus        = uint8(0)
)

func TestValidateAndParseFTDCRequest(t *testing.T) {
	encodedAndAbi := loadEncodedAndAbi(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	reqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: testWalletAddress,
		PublicKeys:    testPublicKeys,
		Threshold:     testThreshold,
	}
	req := testutil.CreateIFtdcHubFtdcAttestationRequest(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIdPair.SourceIdEncoded, testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(t, reqBody))

	t.Run("Valid encodedReq", func(t *testing.T) {
		data, err := validateAndParseFTDCRequest[connector.IPMWMultisigAccountConfiguredRequestBody](req, encodedAndAbi)
		assertNoErrorWithValue(t, err, data, reqBody)
	})

	t.Run("Invalid encodedReq - validation fails", func(t *testing.T) {
		invalidReq := testutil.CreateIFtdcHubFtdcAttestationRequest(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIdPair.SourceIdEncoded, []byte{})
		_, err := validateAndParseFTDCRequest[connector.IPMWMultisigAccountConfiguredRequestBody](invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Invalid attestation type/source id", func(t *testing.T) {
		invalidReq := testutil.CreateIFtdcHubFtdcAttestationRequest(t, [32]byte{0xFF}, encodedAndAbi.SourceIdPair.SourceIdEncoded, testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(t, reqBody))
		_, err := validateAndParseFTDCRequest[connector.IPMWMultisigAccountConfiguredRequestBody](invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Invalid ABI decode", func(t *testing.T) {
		invalidReq := testutil.CreateIFtdcHubFtdcAttestationRequest(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIdPair.SourceIdEncoded, []byte{0xFF, 0xFF})
		_, err := validateAndParseFTDCRequest[connector.IPMWMultisigAccountConfiguredRequestBody](invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func TestHandleVerifierResult(t *testing.T) {
	encodedAndAbi := loadEncodedAndAbi(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	verifierResp := connector.IPMWMultisigAccountConfiguredResponseBody{
		Status:   testStatus,
		Sequence: testSequence,
	}

	t.Run("Success", func(t *testing.T) {
		resp, bytes, err := handleVerifierResult(nil, verifierResp, encodedAndAbi)
		assertNoErrorWithValue(t, err, resp, resp)
		require.NotNil(t, bytes)
		require.Equal(t, testutil.DecodeFTDCPMWMultisigAccountConfiguredResponse(t, bytes), verifierResp)
	})

	t.Run("Verifier error", func(t *testing.T) {
		verifierErr := errors.New("verifier failed")
		_, _, err := handleVerifierResult(verifierErr, verifierResp, encodedAndAbi)
		assertHumaError(t, err, http.StatusInternalServerError)
	})
}

func TestValidatePrepareRequest(t *testing.T) {
	encodedAndAbi := loadEncodedAndAbi(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	validReq := testutil.InternalFTDCRequest(t,
		encodedAndAbi.AttestationTypePair.AttestationTypeEncoded,
		encodedAndAbi.SourceIdPair.SourceIdEncoded,
		attestationtypes.PMWMultisigAccountRequestBody{
			WalletAddress: testWalletAddress,
			PublicKeys:    []hexutil.Bytes{testPublicKeys[0]},
			Threshold:     testThreshold,
		},
	)

	t.Run("Valid", func(t *testing.T) {
		err := validatePrepareRequest(validReq, encodedAndAbi)
		require.NoError(t, err)
	})

	t.Run("Validation error", func(t *testing.T) {
		invalidReq := testutil.InternalFTDCRequest(t,
			encodedAndAbi.AttestationTypePair.AttestationTypeEncoded,
			encodedAndAbi.SourceIdPair.SourceIdEncoded,
			attestationtypes.PMWMultisigAccountRequestBody{},
		)
		err := validatePrepareRequest(invalidReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusBadRequest)
	})

	t.Run("Attestation/source mismatch", func(t *testing.T) {
		mismatchReq := validReq
		mismatchReq.FTDCHeader.AttestationType = [32]byte{0xFF}
		err := validatePrepareRequest(mismatchReq, encodedAndAbi)
		assertHumaError(t, err, http.StatusInternalServerError)
	})
}

func TestPrepareRequestBody(t *testing.T) {
	encodedAndAbi := loadEncodedAndAbi(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	req := connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: testWalletAddress,
		PublicKeys:    testPublicKeys,
		Threshold:     testThreshold,
	}

	t.Run("Valid", func(t *testing.T) {
		resp, err := prepareRequestBody(req, encodedAndAbi)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Body)
		require.NotEmpty(t, resp.Body.RequestBody)
		require.Equal(t, testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(t, req), []byte(resp.Body.RequestBody))
	})

	t.Run("Encoding error", func(t *testing.T) {
		badConfig := &config.EncodedAndAbi{AbiPair: config.AbiArgPair{
			Request:  abi.Argument{},
			Response: abi.Argument{},
		}}
		resp, err := prepareRequestBody(req, badConfig)
		assertHumaError(t, err, http.StatusBadRequest)
		require.Nil(t, resp)
	})
}

func TestPrepareResponseBody(t *testing.T) {
	encodedAndAbi := loadEncodedAndAbi(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	reqBody := testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(t, connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: testWalletAddress,
		PublicKeys:    testPublicKeys,
		Threshold:     testThreshold,
	})
	validRequest := testutil.FTDCRequestEncoded(t, encodedAndAbi.AttestationTypePair.AttestationTypeEncoded, encodedAndAbi.SourceIdPair.SourceIdEncoded, reqBody)
	expectedResp := connector.IPMWMultisigAccountConfiguredResponseBody{
		Status:   testStatus,
		Sequence: testSequence,
	}

	t.Run("Valid", func(t *testing.T) {
		resp, err := prepareResponseBody(
			context.Background(),
			validRequest,
			validateAndVerifyEncodedPMWMultisigAccountRequest,
			attestationtypes.MultiSigToExternal,
			encodedAndAbi,
			&mockVerifier{
				verifyFunc: func(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
					return expectedResp, nil
				},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, attestationtypes.MultiSigToExternal(expectedResp), resp.Body.ResponseData)
		require.NotEmpty(t, resp.Body.ResponseBody)
		require.Equal(t, testutil.DecodeFTDCPMWMultisigAccountConfiguredResponse(t, resp.Body.ResponseBody), expectedResp)
	})

	t.Run("Verifier error", func(t *testing.T) {
		_, err := prepareResponseBody(
			context.Background(),
			validRequest,
			validateAndVerifyEncodedPMWMultisigAccountRequest,
			attestationtypes.MultiSigToExternal,
			encodedAndAbi,
			&mockVerifier{
				verifyFunc: func(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
					return connector.IPMWMultisigAccountConfiguredResponseBody{}, errors.New("fail")
				},
			},
		)
		require.Error(t, err)
	})
}

type mockVerifier struct {
	verifyFunc func(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error)
}

func (m *mockVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	return m.verifyFunc(ctx, req)
}

func loadEncodedAndAbi(t *testing.T, attestationType connector.AttestationType, sourceId config.SourceName) *config.EncodedAndAbi {
	t.Helper()
	encodedAndAbi, err := config.LoadEncodedAndAbi(config.EnvConfig{
		ApiKeys:         nil,
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

func assertNoErrorWithValue[T any](t *testing.T, err error, actual, expected T) {
	t.Helper()
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
