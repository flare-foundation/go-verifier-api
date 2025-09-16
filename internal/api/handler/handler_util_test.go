package handler

import (
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var (
	testAccountAddress = "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	testPublicKeys     = [][]byte{{0x01, 0x02}}
	testThreshold      = uint64(2)
)

func TestPrepareRequestBody(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	attBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     testPublicKeys,
		Threshold:      testThreshold,
	}
	reqBody := testhelper.PMWMultisigAccountConfiguredRequestBody(attBody)

	t.Run("Valid encodedReq", func(t *testing.T) {
		req := testhelper.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBody)
		_, err := PrepareRequestBody(req, encodedAndABI)
		require.NoError(t, err)
	})
	t.Run("Invalid encodedReq - validation fails", func(t *testing.T) {
		reqBodyMod := reqBody
		reqBodyMod.PublicKeys = append(reqBodyMod.PublicKeys, hexutil.Bytes{})
		invalidReq := testhelper.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBodyMod)
		_, err := PrepareRequestBody(invalidReq, encodedAndABI)
		assertHumaError(t, err, http.StatusBadRequest)
	})
	t.Run("Invalid ABI encode", func(t *testing.T) {
		req := testhelper.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBody)
		encodedAndABICopy := encodedAndABI
		encodedAndABICopy.ABIPair.Request = abi.Argument{}
		_, err := PrepareRequestBody(req, encodedAndABI)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func TestValidateSystemAndRequestAttestationNameAndSourceID(t *testing.T) {
	attestationTypePair := config.AttestationTypeEncodedPair{
		AttestationType:        "TestType",
		AttestationTypeEncoded: common.HexToHash("0x1234"),
	}
	sourceIDPair := config.SourceIDEncodedPair{
		SourceID:        "TestSource",
		SourceIDEncoded: common.HexToHash("0x5678"),
	}
	cfg := &config.EncodedAndABI{
		SourceIDPair:        sourceIDPair,
		AttestationTypePair: attestationTypePair,
		ABIPair:             config.ABIArgPair{},
	}
	// Matching values
	err := ValidateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		sourceIDPair.SourceIDEncoded.Hex(),
	)
	require.NoError(t, err)
	// Mismatched attestation type
	err = ValidateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		"0xdeadbeef",
		sourceIDPair.SourceIDEncoded.Hex(),
	)
	assertHumaError(t, err, http.StatusBadRequest)
	require.Contains(t, err.Error(), "attestation type and source id combination not supported")
	// Mismatched source id
	err = ValidateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		"0xdeadbeef",
	)
	assertHumaError(t, err, http.StatusBadRequest)
	require.Contains(t, err.Error(), "attestation type and source id combination not supported")
}

func TestDecodeRequest(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	baseReqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     testPublicKeys,
		Threshold:      testThreshold,
	}
	t.Run("Valid", func(t *testing.T) {
		encoded := testhelper.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		decoded, err := DecodeRequest[attestationtypes.PMWMultisigAccountConfiguredRequestBody](encoded, encodedAndABI)
		require.NoError(t, err)
		require.Equal(t, testAccountAddress, decoded.AccountAddress)
		require.Equal(t, testPublicKeys[0], []byte(decoded.PublicKeys[0]))
		require.Equal(t, testThreshold, decoded.Threshold)
	})
	t.Run("Invalid", func(t *testing.T) {
		encoded := testhelper.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		invalidBody := append([]byte(nil), encoded...)
		invalidBody = append(invalidBody, 'a', 'a')
		_, err := DecodeRequest[attestationtypes.PMWMultisigAccountConfiguredRequestBody](invalidBody, encodedAndABI)
		assertHumaError(t, err, http.StatusBadRequest)
	})
}

func TestEncodeResponse(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	t.Run("Valid", func(t *testing.T) {
		resp := connector.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(attestationtypes.PMWMultisigAccountStatusOK),
			Sequence: 10136106,
		}
		encoded, err := EncodeResponse(resp, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[connector.IPMWMultisigAccountConfiguredResponseBody](encodedAndABI.ABIPair.Response, encoded)
		require.NoError(t, err)
		require.Equal(t, resp, decoded)
	})
	t.Run("Unserializable type", func(t *testing.T) {
		type Temp struct {
			t int
		}
		resp := Temp{t: 1}
		_, err := EncodeResponse(resp, encodedAndABI)
		assertHumaError(t, err, http.StatusInternalServerError)
	})
}

func TestEncodeRequest(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
	t.Run("Valid", func(t *testing.T) {
		req := connector.IPMWMultisigAccountConfiguredRequestBody{
			AccountAddress: testAccountAddress,
			PublicKeys:     testPublicKeys,
			Threshold:      testThreshold,
		}
		encoded, err := EncodeRequest(req, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[connector.IPMWMultisigAccountConfiguredRequestBody](encodedAndABI.ABIPair.Request, encoded)
		require.NoError(t, err)
		require.Equal(t, req, decoded)
	})
	t.Run("Unserializable type", func(t *testing.T) {
		type Temp struct {
			t int
		}
		req := Temp{t: 1}
		_, err := EncodeRequest(req, encodedAndABI)
		assertHumaError(t, err, http.StatusInternalServerError)
	})
}

func assertHumaError(t *testing.T, err error, expectedStatus int) {
	t.Helper()
	var herr *huma.ErrorModel
	require.Error(t, err)
	require.ErrorAs(t, err, &herr)
	require.Equal(t, expectedStatus, herr.Status)
}

func loadTestEncodedAndABI(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName) *config.EncodedAndABI {
	t.Helper()
	encodedAndABI, err := config.LoadEncodedAndABI(config.EnvConfig{
		APIKeys:         nil,
		AttestationType: attestationType,
		SourceID:        sourceID,
	})
	require.NoError(t, err)
	return &encodedAndABI
}
