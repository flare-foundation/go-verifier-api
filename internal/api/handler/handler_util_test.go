package handler

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"

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
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP)
	attBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     testPublicKeys,
		Threshold:      testThreshold,
	}
	reqBody := helpers.PMWMultisigAccountConfiguredRequestBody(t, attBody)

	t.Run("valid encodedReq", func(t *testing.T) {
		req := helpers.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBody)
		val, err := prepareRequestBody(req, encodedAndABI)
		require.NoError(t, err)
		require.NotNil(t, val)
	})
	t.Run("invalid encodedReq - validation fails", func(t *testing.T) {
		reqBodyMod := reqBody
		reqBodyMod.PublicKeys = append(reqBodyMod.PublicKeys, hexutil.Bytes{})
		invalidReq := helpers.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBodyMod)
		val, err := prepareRequestBody(invalidReq, encodedAndABI)
		require.Nil(t, val)
		require.ErrorContains(t, err, "converting request body to data failed: public key at index 1 is empty")
	})
	t.Run("invalid ABI encode", func(t *testing.T) {
		req := helpers.CreateAttestationRequestData(t, encodedAndABI.AttestationTypePair.AttestationTypeEncoded, encodedAndABI.SourceIDPair.SourceIDEncoded, reqBody)
		encodedAndABICopy := encodedAndABI
		encodedAndABICopy.ABIPair.Request = abi.Argument{}
		val, err := prepareRequestBody(req, encodedAndABI)
		require.ErrorContains(t, err, "encoding request data failed: encoding type connector.IPMWMultisigAccountConfiguredRequestBody: abi: cannot use struct as type ptr as argument")
		require.Nil(t, val)
	})
}

func TestResolve(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP)
	attBodyInvalid := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     [][]byte{}, // empty slice triggers "min=1" validation
		Threshold:      0,          // violates "gte=1"
	}
	reqBodyInvalid := helpers.PMWMultisigAccountConfiguredRequestBody(t, attBodyInvalid)

	req := types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{
		AttestationType: encodedAndABI.AttestationTypePair.AttestationTypeEncoded,
		SourceID:        encodedAndABI.SourceIDPair.SourceIDEncoded,
		RequestData:     reqBodyInvalid,
	}

	errs := req.Resolve(nil)
	require.NotEmpty(t, errs)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "PublicKeys")
	require.Contains(t, errs[0].Error(), "Threshold")
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
	err := validateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		sourceIDPair.SourceIDEncoded.Hex(),
	)
	require.NoError(t, err)
	// Mismatched attestation type
	err = validateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		"0xdeadbeef",
		sourceIDPair.SourceIDEncoded.Hex(),
	)
	require.ErrorContains(t, err, "attestation type and source id combination not supported")
	// Mismatched source id
	err = validateSystemAndRequestAttestationNameAndSourceID(
		cfg,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		"0xdeadbeef",
	)
	require.ErrorContains(t, err, "attestation type and source id combination not supported")
}

func TestDecodeRequest(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP)
	baseReqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     testPublicKeys,
		Threshold:      testThreshold,
	}
	t.Run("valid", func(t *testing.T) {
		encoded := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		decoded, err := decodeRequest[types.PMWMultisigAccountConfiguredRequestBody](encoded, encodedAndABI)
		require.NoError(t, err)
		require.Equal(t, testAccountAddress, decoded.AccountAddress)
		require.Equal(t, testPublicKeys[0], []byte(decoded.PublicKeys[0]))
		require.Equal(t, testThreshold, decoded.Threshold)
	})
	t.Run("invalid", func(t *testing.T) {
		encoded := helpers.EncodeRequestBody(t, connector.PMWMultisigAccountConfigured, baseReqBody)
		invalidBody := append([]byte(nil), encoded...)
		invalidBody = append(invalidBody, 'a', 'a')
		val, err := decodeRequest[types.PMWMultisigAccountConfiguredRequestBody](invalidBody, encodedAndABI)
		require.ErrorContains(t, err, "initial data not equal to decoded and encoded data")
		require.Equal(t, types.PMWMultisigAccountConfiguredRequestBody{}, val)
	})
}

func TestEncodeResponse(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP)
	t.Run("valid", func(t *testing.T) {
		resp := connector.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(types.PMWMultisigAccountStatusOK),
			Sequence: 10136106,
		}
		encoded, err := encodeResponse(resp, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[connector.IPMWMultisigAccountConfiguredResponseBody](encodedAndABI.ABIPair.Response, encoded)
		require.NoError(t, err)
		require.Equal(t, resp, decoded)
	})
	t.Run("unserializable type", func(t *testing.T) {
		type Temp struct {
			t int
		}
		resp := Temp{t: 1}
		val, err := encodeResponse(resp, encodedAndABI)
		require.ErrorContains(t, err, "encoding response data failed: encoding type handler.Temp: field status for tuple not found in the given struct")
		require.Nil(t, val)
	})
}

func TestEncodeRequest(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t, connector.PMWMultisigAccountConfigured, config.SourceTestXRP)
	t.Run("valid", func(t *testing.T) {
		req := connector.IPMWMultisigAccountConfiguredRequestBody{
			AccountAddress: testAccountAddress,
			PublicKeys:     testPublicKeys,
			Threshold:      testThreshold,
		}
		encoded, err := encodeRequest(req, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[connector.IPMWMultisigAccountConfiguredRequestBody](encodedAndABI.ABIPair.Request, encoded)
		require.NoError(t, err)
		require.Equal(t, req, decoded)
	})
	t.Run("unserializable type", func(t *testing.T) {
		type Temp struct {
			t int
		}
		req := Temp{t: 1}
		val, err := encodeRequest(req, encodedAndABI)
		require.ErrorContains(t, err, "encoding request data failed: encoding type handler.Temp: field accountAddress for tuple not found in the given struct")
		require.Nil(t, val)
	})
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
