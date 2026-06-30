package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	feeproofxrp "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/xrp"
	multisigxrp "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var (
	testAccountAddress = "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	testPublicKeys     = [][]byte{{0x01, 0x02}}
	testThreshold      = uint64(2)
)

func TestPrepareRequestBody(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t)
	attBody := fdc2.IPMWMultisigAccountConfiguredRequestBody{
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
		val, err := prepareRequestBody(req, encodedAndABICopy)
		require.ErrorContains(t, err, "encoding request data failed: uninitialized abi argument: zero abi.Type")
		require.Nil(t, val)
	})
}

func TestResolve(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t)
	attBodyInvalid := fdc2.IPMWMultisigAccountConfiguredRequestBody{
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
	encodedAndABI := loadTestEncodedAndABI(t)
	baseReqBody := fdc2.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: testAccountAddress,
		PublicKeys:     testPublicKeys,
		Threshold:      testThreshold,
	}
	t.Run("valid", func(t *testing.T) {
		encoded := helpers.EncodeRequestBody(t, fdc2.PMWMultisigAccountConfigured, baseReqBody)
		decoded, err := decodeRequest[types.PMWMultisigAccountConfiguredRequestBody](encoded, encodedAndABI)
		require.NoError(t, err)
		require.Equal(t, testAccountAddress, decoded.AccountAddress)
		require.Equal(t, testPublicKeys[0], []byte(decoded.PublicKeys[0]))
		require.Equal(t, testThreshold, decoded.Threshold)
	})
	t.Run("invalid", func(t *testing.T) {
		encoded := helpers.EncodeRequestBody(t, fdc2.PMWMultisigAccountConfigured, baseReqBody)
		invalidBody := append([]byte(nil), encoded...)
		invalidBody = append(invalidBody, 'a', 'a')
		val, err := decodeRequest[types.PMWMultisigAccountConfiguredRequestBody](invalidBody, encodedAndABI)
		require.ErrorContains(t, err, "initial data not equal to decoded and encoded data")
		require.Equal(t, types.PMWMultisigAccountConfiguredRequestBody{}, val)
	})
}

func TestEncodeResponse(t *testing.T) {
	encodedAndABI := loadTestEncodedAndABI(t)
	t.Run("valid", func(t *testing.T) {
		resp := fdc2.IPMWMultisigAccountConfiguredResponseBody{
			Status:   uint8(types.PMWMultisigAccountStatusOK),
			Sequence: 10136106,
		}
		encoded, err := encodeResponse(resp, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[fdc2.IPMWMultisigAccountConfiguredResponseBody](encodedAndABI.ABIPair.Response, encoded)
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
	encodedAndABI := loadTestEncodedAndABI(t)
	t.Run("valid", func(t *testing.T) {
		req := fdc2.IPMWMultisigAccountConfiguredRequestBody{
			AccountAddress: testAccountAddress,
			PublicKeys:     testPublicKeys,
			Threshold:      testThreshold,
		}
		encoded, err := encodeRequest(req, encodedAndABI)
		require.NoError(t, err)
		decoded, err := structs.Decode[fdc2.IPMWMultisigAccountConfiguredRequestBody](encodedAndABI.ABIPair.Request, encoded)
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

func loadTestEncodedAndABI(t *testing.T) *config.EncodedAndABI {
	t.Helper()
	attestationType := fdc2.PMWMultisigAccountConfigured
	encodedAndABI, err := config.LoadEncodedAndABI(config.EnvConfig{
		APIKeys:         nil,
		AttestationType: attestationType,
		SourceID:        config.SourceTestXRP,
	})
	require.NoError(t, err)
	return &encodedAndABI
}

func TestClassifyVerifyError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		// 400 — bad request
		{
			name:           "ErrBatchRangeTooLarge",
			err:            fmt.Errorf("range exceeds max: %w", feeproofxrp.ErrBatchRangeTooLarge),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrReissueLimitExceeded",
			err:            fmt.Errorf("nonce 100: %w (cap 32)", feeproofxrp.ErrReissueLimitExceeded),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrInvalidRequest (multisig)",
			err:            fmt.Errorf("too many keys: %w", multisigxrp.ErrInvalidRequest),
			expectedStatus: http.StatusBadRequest,
		},
		// 422 — PMW errors
		{
			name:           "ErrRPCNonSuccess",
			err:            fmt.Errorf("rpc non-success: %w", client.ErrRPCNonSuccess),
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "ErrRecordNotFound",
			err:            fmt.Errorf("record not found: %w", db.ErrRecordNotFound),
			expectedStatus: http.StatusUnprocessableEntity,
		},
		// 422 — TEE data validation
		{
			name:           "ErrTEEDataValidation",
			err:            fmt.Errorf("challenge mismatch: %w", verifier.ErrTEEDataValidation),
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "ErrInvalidInput",
			err:            fmt.Errorf("rpc call failed: %w", verifiertypes.ErrInvalidInput),
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "ErrActionResultNotFound",
			err:            fmt.Errorf("action result not ready: %w", verifier.ErrActionResultNotFound),
			expectedStatus: http.StatusServiceUnavailable,
		},
		// 503 — PMW infrastructure errors
		{
			name:           "ErrFetchAccountInfo",
			err:            fmt.Errorf("account info failed: %w", client.ErrFetchAccountInfo),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrDatabase",
			err:            fmt.Errorf("db failed: %w", db.ErrDatabase),
			expectedStatus: http.StatusServiceUnavailable,
		},
		// 503 — request deadline / cancellation
		{
			name:           "context deadline exceeded",
			err:            fmt.Errorf("verifier work timed out: %w", context.DeadlineExceeded),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "context canceled",
			err:            fmt.Errorf("client disconnected: %w", context.Canceled),
			expectedStatus: http.StatusServiceUnavailable,
		},
		// 503 — TEE infrastructure errors
		{
			name:           "ErrNetwork",
			err:            fmt.Errorf("rpc call failed: %w", verifiertypes.ErrNetwork),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrRPC",
			err:            fmt.Errorf("rpc call failed: %w", verifiertypes.ErrRPC),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrContext",
			err:            fmt.Errorf("context error: %w", verifiertypes.ErrContext),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrUnknown",
			err:            fmt.Errorf("unknown error: %w", verifiertypes.ErrUnknown),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrHTTPFetch",
			err:            fmt.Errorf("HTTP failed: %w", fetcher.ErrHTTPFetch),
			expectedStatus: http.StatusServiceUnavailable,
		},
		// 500 — default
		{
			name:           "unknown error falls to 500",
			err:            errors.New("something unexpected"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyVerifyError("", tt.err)
			var statusErr huma.StatusError
			require.ErrorAs(t, result, &statusErr)
			require.Equal(t, tt.expectedStatus, statusErr.GetStatus())
		})
		t.Run(tt.name+" with reqID", func(t *testing.T) {
			result := classifyVerifyError("test1234", tt.err)
			var statusErr huma.StatusError
			require.ErrorAs(t, result, &statusErr)
			require.Equal(t, tt.expectedStatus, statusErr.GetStatus())
			// reqID must not leak into the HTTP response body.
			require.NotContains(t, statusErr.Error(), "test1234")
		})
	}
}

// blockingVerifier blocks until its context is cancelled, modelling a hung
// dependency (slow DB or RPC).
type blockingVerifier struct{}

func (blockingVerifier) Verify(ctx context.Context, _ int) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

// instantVerifier returns immediately, modelling a healthy dependency.
type instantVerifier struct{}

func (instantVerifier) Verify(_ context.Context, req int) (int, error) {
	return req + 1, nil
}

func TestVerifyWithDeadline(t *testing.T) {
	t.Run("times out a hung verifier", func(t *testing.T) {
		_, err := verifyWithDeadline[int, int](context.Background(), blockingVerifier{}, 0, 10*time.Millisecond)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("returns a fast verifier's result", func(t *testing.T) {
		got, err := verifyWithDeadline[int, int](context.Background(), instantVerifier{}, 41, time.Second)
		require.NoError(t, err)
		require.Equal(t, 42, got)
	})
}
