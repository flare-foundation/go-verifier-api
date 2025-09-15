package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	api "github.com/flare-foundation/go-verifier-api/internal/api_test"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestPMWMultisig_PrepareRequestBody(t *testing.T) {
	setup := api.SetupServer(t, connector.PMWMultisigAccountConfigured, config.SourceXRP, config.EnvConfig{
		RPCURL: "https://s.altnet.rippletest.net:51234",
	})
	defer setup.Stop()

	// /prepareRequestBody

	t.Run("prepareRequestBody: Valid request", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		reqData := testhelper.PMWMultisigAccountConfiguredRequestBody("rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", []hexutil.Bytes{pubkey1, pubkey2, pubkey3}, 1)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.Post[types.AttestationRequestEncoded](t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		attBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, request.RequestData.AccountAddress, [][]byte{pubkey1, pubkey2, pubkey3}, request.RequestData.Threshold)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody: Bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{}, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	t.Run("prepareRequestBody: Empty public key", func(t *testing.T) {
		reqData := testhelper.PMWMultisigAccountConfiguredRequestBody("rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", []hexutil.Bytes{{}}, 1)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	// /prepareResponseBody

	t.Run("prepareResponseBody: Correctly created multisig wallet", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{pubkey1, pubkey2, pubkey3}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]](t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), response.ResponseData.PMWMultisigAccountStatus)
		require.Equal(t, uint64(10136106), response.ResponseData.Sequence)
	})

	t.Run("prepareResponseBody: Invalid sourceId", func(t *testing.T) {
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123123"), reqBody)

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareResponseBody: Invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte{})
		request.RequestBody = []byte{}

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	// /verify

	t.Run("verify: Correctly created multisig wallet", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{pubkey1, pubkey2, pubkey3}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		result := testhelper.DecodeFTDCPMWMultisigAccountConfiguredResponse(t, response.ResponseBody)
		require.NoError(t, err)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
		require.Equal(t, uint64(10136106), result.Sequence)
	})

	t.Run("verify: Missing pubkey in request", func(t *testing.T) {
		pubkey1, pubkey2, _ := pubKeysForMultisig(t)
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{pubkey1, pubkey2}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		result := testhelper.DecodeFTDCPMWMultisigAccountConfiguredResponse(t, response.ResponseBody)
		require.NoError(t, err)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusERROR), result.Status)
		require.Equal(t, uint64(0), result.Sequence)
	})

	t.Run("verify: Invalid sourceId", func(t *testing.T) {
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123123"), reqBody)

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid attestation type", func(t *testing.T) {
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{}, 1)
		request := testhelper.CreateAttestationRequest(t, [32]byte{0xFF}, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte{})
		request.RequestBody = []byte{}

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	t.Run("verify: Invalid address - failed to get account info", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		reqBody := testhelper.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77a", [][]byte{pubkey1, pubkey2, pubkey3}, 1)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		_, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.Error(t, err)
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
