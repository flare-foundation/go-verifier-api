package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/flare-foundation/go-verifier-api/internal/tests/server"
	"github.com/stretchr/testify/require"
)

func TestPMWMultisigAccountConfigured(t *testing.T) {
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	setup := server.SetupServer(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP, config.EnvConfig{
		RPCURL: "https://s.altnet.rippletest.net:51234",
	})
	defer setup.Stop()

	pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
	testAddress := "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL"
	pubKeys := [][]byte{pubkey1, pubkey2, pubkey3}
	mSigThr := uint64(1)
	baseReqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAddress,
		PublicKeys:     pubKeys,
		Threshold:      mSigThr,
	}
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := helpers.PMWMultisigAccountConfiguredRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := helpers.Post[types.AttestationRequestEncoded](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := request.RequestData.ToInternal()
		require.NoError(t, err)

		attBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: empty public key", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.PublicKeys = [][]byte{{}}
		reqData := helpers.PMWMultisigAccountConfiguredRequestBody(t, modifiedReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Prepare request failed: converting request body to data failed: public key at index 0 is empty")
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := helpers.PMWMultisigAccountConfiguredRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x123123"), reqData)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := helpers.Post[types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), response.ResponseData.PMWMultisigAccountStatus)
		require.Equal(t, uint64(10136106), response.ResponseData.Sequence)
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("prepareResponseBody: invalid address - failed to get account info", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.AccountAddress = modifiedReqBody.AccountAddress[4:] // Remove 4 for chars.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed: XRP RPC returned non-success status for account rSYbeGm77aYjnvuHVnBwZ1TkLnu1UL: error")
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("verify: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := helpers.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		result := helpers.DecodeResponseBody[connector.IPMWMultisigAccountConfiguredResponseBody](t, connector.PMWMultisigAccountConfigured, response.ResponseBody)
		require.NoError(t, err)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
		require.Equal(t, uint64(10136106), result.Sequence)
	})
	t.Run("verify: missing pubkey in request", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.PublicKeys = modifiedReqBody.PublicKeys[:2] // Remove last public key.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := helpers.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		result := helpers.DecodeResponseBody[connector.IPMWMultisigAccountConfiguredResponseBody](t, connector.PMWMultisigAccountConfigured, response.ResponseBody)
		require.NoError(t, err)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusERROR), result.Status)
		require.Equal(t, uint64(0), result.Sequence)
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid attestation type", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		request := helpers.CreateAttestationRequest(t, [32]byte{0xFF}, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("verify: invalid address - failed to get account info", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.AccountAddress = modifiedReqBody.AccountAddress[4:] // Remove 4 for chars.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed: XRP RPC returned non-success status for account rSYbeGm77aYjnvuHVnBwZ1TkLnu1UL: error")
	})
}

func TestPMWMultisigAccountConfigured_ServiceUnavailable(t *testing.T) {
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	setup := server.SetupServer(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP, config.EnvConfig{
		RPCURL: "http://localhost:1", // Unreachable RPC URL to trigger ErrGetAccountInfo.
	})
	defer setup.Stop()

	pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
	reqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
		PublicKeys:     [][]byte{pubkey1, pubkey2, pubkey3},
		Threshold:      uint64(1),
	}

	t.Run("verify: RPC unreachable returns 503", func(t *testing.T) {
		encodedReqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, reqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, encodedReqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusServiceUnavailable, "Verification failed")
	})

	t.Run("prepareResponseBody: RPC unreachable returns 503", func(t *testing.T) {
		encodedReqBody := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, reqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, encodedReqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusServiceUnavailable, "Verification failed")
	})
}

func pubKeysForMultisig(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()
	pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
	require.NoError(t, err)
	pubkey3, err := hexutil.Decode("0x76e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb")
	require.NoError(t, err)
	return pubkey1, pubkey2, pubkey3
}
