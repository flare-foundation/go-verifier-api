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
	multisigutxobtc "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc"
	btcclient "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc/client"
	paymentstatusbtc "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/nodechain"
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
		{
			name:           "ErrInvalidRequest (utxo multisig BTC)",
			err:            fmt.Errorf("bad anchor set: %w", multisigutxobtc.ErrInvalidRequest),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrMissingTransactionID (payment-status BTC)",
			err:            fmt.Errorf("no locator: %w", paymentstatusbtc.ErrMissingTransactionID),
			expectedStatus: http.StatusBadRequest,
		},
		// 422 — PMW errors
		{
			name:           "ErrRPCNonSuccess",
			err:            fmt.Errorf("rpc non-success: %w", client.ErrRPCNonSuccess),
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			// A transient node status (tooBusy/noNetwork/...) is retryable, NOT a
			// terminal 422 like a deterministic actNotFound.
			name:           "ErrRPCTransient",
			err:            fmt.Errorf("rpc transient: %w", client.ErrRPCTransient),
			expectedStatus: http.StatusServiceUnavailable,
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
		{
			name:           "ErrNetworkMismatch (payment-status BTC)",
			err:            fmt.Errorf("wrong chain: %w", paymentstatusbtc.ErrNetworkMismatch),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrNetworkUnverified (payment-status BTC)",
			err:            fmt.Errorf("probe in flight: %w", paymentstatusbtc.ErrNetworkUnverified),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrNodeUnavailable (payment-status BTC node)",
			err:            fmt.Errorf("node down: %w", nodechain.ErrNodeUnavailable),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrNetworkMismatch (utxo multisig BTC)",
			err:            fmt.Errorf("wrong chain: %w", multisigutxobtc.ErrNetworkMismatch),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrNetworkUnverified (utxo multisig BTC)",
			err:            fmt.Errorf("probe in flight: %w", multisigutxobtc.ErrNetworkUnverified),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrFetchChainInfo (BTC)",
			err:            fmt.Errorf("node unreachable: %w", btcclient.ErrFetchChainInfo),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrGetTxOut (BTC)",
			err:            fmt.Errorf("gettxout failed: %w", btcclient.ErrGetTxOut),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrDataSource",
			err:            fmt.Errorf("cannot decode event: %w (boom)", db.ErrDataSource),
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

func TestClassifyVerifyStatus(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus string
	}{
		// REJECTED — deterministic (mirrors classifyVerifyError's 400/422 cases)
		{"ErrBatchRangeTooLarge", fmt.Errorf("range exceeds max: %w", feeproofxrp.ErrBatchRangeTooLarge), types.StatusRejected},
		{"ErrReissueLimitExceeded", fmt.Errorf("nonce 100: %w (cap 32)", feeproofxrp.ErrReissueLimitExceeded), types.StatusRejected},
		{"ErrInvalidRequest (multisig)", fmt.Errorf("too many keys: %w", multisigxrp.ErrInvalidRequest), types.StatusRejected},
		{"ErrMissingPayEvent", fmt.Errorf("no pay event: %w", feeproofxrp.ErrMissingPayEvent), types.StatusRejected},
		{"ErrMissingTransaction", fmt.Errorf("no xrp tx: %w", feeproofxrp.ErrMissingTransaction), types.StatusRejected},
		{"ErrRPCNonSuccess", fmt.Errorf("rpc non-success: %w", client.ErrRPCNonSuccess), types.StatusRejected},
		{"ErrRecordNotFound", fmt.Errorf("record not found: %w", db.ErrRecordNotFound), types.StatusRejected},
		{"ErrTEEDataValidation", fmt.Errorf("challenge mismatch: %w", verifier.ErrTEEDataValidation), types.StatusRejected},
		{"ErrTEEChallengeMismatch", fmt.Errorf("challenge does not match: %w: %w", verifier.ErrTEEChallengeMismatch, verifier.ErrTEEDataValidation), types.StatusRejected},
		{"ErrTEEChainIDMismatch", fmt.Errorf("chainID does not match: %w: %w", verifier.ErrTEEChainIDMismatch, verifier.ErrTEEDataValidation), types.StatusRejected},
		{"ErrTEEProxySignerMismatch", fmt.Errorf("proxy signer does not match: %w: %w", verifier.ErrTEEProxySignerMismatch, verifier.ErrTEEDataValidation), types.StatusRejected},
		{"ErrTEESigningPolicyHash", fmt.Errorf("failed to validate initial signing policy hash: %w: %w", verifier.ErrTEESigningPolicyHash, verifier.ErrTEEDataValidation), types.StatusRejected},
		{"ErrTEEAttestationInvalid", fmt.Errorf("%w: cannot validate certificate signature", verifier.ErrTEEAttestationInvalid), types.StatusRejected},
		{"ErrTEEResponseMalformed", fmt.Errorf("%w: TEE challenge result data is empty", verifier.ErrTEEResponseMalformed), types.StatusRejected},
		{"ErrInvalidInput", fmt.Errorf("bad input: %w", verifiertypes.ErrInvalidInput), types.StatusRejected},
		// RETRY — transient (mirrors classifyVerifyError's 503 cases)
		{"context deadline exceeded", fmt.Errorf("verifier work timed out: %w", context.DeadlineExceeded), types.StatusRetry},
		{"context canceled", fmt.Errorf("client disconnected: %w", context.Canceled), types.StatusRetry},
		{"ErrFetchAccountInfo", fmt.Errorf("account info failed: %w", client.ErrFetchAccountInfo), types.StatusRetry},
		{"ErrRPCTransient", fmt.Errorf("too busy: %w for account rX: tooBusy", client.ErrRPCTransient), types.StatusRetry},
		{"ErrDatabase", fmt.Errorf("db failed: %w", db.ErrDatabase), types.StatusRetry},
		{"ErrDataSource", fmt.Errorf("cannot decode event: %w (boom)", db.ErrDataSource), types.StatusRetry},
		{"ErrNetwork", fmt.Errorf("rpc call failed: %w", verifiertypes.ErrNetwork), types.StatusRetry},
		{"ErrRPC", fmt.Errorf("rpc call failed: %w", verifiertypes.ErrRPC), types.StatusRetry},
		{"ErrContext", fmt.Errorf("context error: %w", verifiertypes.ErrContext), types.StatusRetry},
		{"ErrUnknown", fmt.Errorf("unknown error: %w", verifiertypes.ErrUnknown), types.StatusRetry},
		{"ErrHTTPFetch", fmt.Errorf("HTTP failed: %w", fetcher.ErrHTTPFetch), types.StatusRetry},
		{"ErrActionResultNotFound", fmt.Errorf("action result not ready: %w", verifier.ErrActionResultNotFound), types.StatusRetry},
		{"ErrTEERevocationUnavailable", fmt.Errorf("attestation revocation check failed: CRL fetch failed: %w", verifier.ErrTEERevocationUnavailable), types.StatusRetry},
		// RETRY — default (mirrors classifyVerifyError's 500 case)
		{"unknown error falls to RETRY", errors.New("something unexpected"), types.StatusRetry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, message := classifyVerifyStatus(tt.err)
			require.Equal(t, tt.expectedStatus, status)
			require.NotEmpty(t, message, "a non-VERIFIED envelope must carry a reason")
			// The safe message must not embed the internal error detail.
			require.NotContains(t, message, tt.err.Error())
		})
	}
}

// TestClassifyVerifyStatusTEEGranularMessages: the specific TEE checks each carry
// their own curated message (so the relay can tell which check failed), the
// specific case wins over the generic one it is chained with, and the generic
// sentinel still matches for any code that keys off ErrTEEDataValidation.
func TestClassifyVerifyStatusTEEGranularMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("challenge does not match: %w: %w", verifier.ErrTEEChallengeMismatch, verifier.ErrTEEDataValidation), "TEE challenge mismatch"},
		{fmt.Errorf("chainID does not match: %w: %w", verifier.ErrTEEChainIDMismatch, verifier.ErrTEEDataValidation), "TEE chain id mismatch"},
		{fmt.Errorf("proxy signer does not match: %w: %w", verifier.ErrTEEProxySignerMismatch, verifier.ErrTEEDataValidation), "TEE proxy signer mismatch"},
		{fmt.Errorf("failed to validate initial signing policy hash: %w: %w", verifier.ErrTEESigningPolicyHash, verifier.ErrTEEDataValidation), "TEE signing policy hash mismatch"},
		{fmt.Errorf("%w: cannot validate certificate signature", verifier.ErrTEEAttestationInvalid), "TEE attestation invalid"},
		{fmt.Errorf("%w: unmarshal TEE result", verifier.ErrTEEResponseMalformed), "TEE response malformed"},
	}
	for _, c := range cases {
		status, message := classifyVerifyStatus(c.err)
		require.Equal(t, types.StatusRejected, status)
		require.Equal(t, c.want, message, "specific TEE reason must win over the generic category")
		// Backward compatibility: the generic sentinel still matches.
		require.ErrorIs(t, c.err, verifier.ErrTEEDataValidation)
	}
	// The generic (unspecific) TEE failure still falls back to the coarse message.
	_, generic := classifyVerifyStatus(fmt.Errorf("some tee issue: %w", verifier.ErrTEEDataValidation))
	require.Equal(t, "TEE data validation failed", generic)

	// Correctness: a transient revocation-check (CRL) fetch failure must be RETRY,
	// never a terminal REJECTED — and it must NOT chain the generic validation
	// sentinel, or it would be swallowed as a terminal "TEE invalid".
	crlErr := fmt.Errorf("attestation revocation check failed: CRL fetch failed: %w", verifier.ErrTEERevocationUnavailable)
	status, message := classifyVerifyStatus(crlErr)
	require.Equal(t, types.StatusRetry, status)
	require.Equal(t, "TEE revocation check unavailable", message)
	require.NotErrorIs(t, crlErr, verifier.ErrTEEDataValidation,
		"a CRL fetch outage must not be classifiable as a terminal TEE validation failure")
}

// TestClassifyVerifyStatusParity guards against the two classifiers drifting: every
// sentinel classifyVerifyError knows about (across ALL attestation types) must be
// handled explicitly by classifyVerifyStatus too — never falling to the generic
// "unexpected error" default — with the same terminal-vs-retryable verdict. If a
// new sentinel is wired into the HTTP classifier but not the /verify envelope,
// adding it here fails until parity is restored.
func TestClassifyVerifyStatusParity(t *testing.T) {
	rejected := []error{
		feeproofxrp.ErrBatchRangeTooLarge,
		feeproofxrp.ErrReissueLimitExceeded,
		feeproofxrp.ErrMissingPayEvent,
		feeproofxrp.ErrMissingTransaction,
		multisigxrp.ErrInvalidRequest,
		multisigutxobtc.ErrInvalidRequest,
		paymentstatusbtc.ErrMissingTransactionID,
		client.ErrRPCNonSuccess,
		db.ErrRecordNotFound,
		verifier.ErrTEEDataValidation,
		verifiertypes.ErrInvalidInput,
	}
	retry := []error{
		context.DeadlineExceeded,
		context.Canceled,
		client.ErrFetchAccountInfo,
		client.ErrFetchServerInfo,
		client.ErrRPCTransient,
		multisigxrp.ErrNetworkMismatch,
		multisigutxobtc.ErrNetworkMismatch,
		multisigutxobtc.ErrNetworkUnverified,
		btcclient.ErrFetchChainInfo,
		btcclient.ErrGetTxOut,
		paymentstatusbtc.ErrNetworkMismatch,
		paymentstatusbtc.ErrNetworkUnverified,
		nodechain.ErrNodeUnavailable,
		db.ErrDatabase,
		db.ErrDataSource,
		verifiertypes.ErrNetwork,
		verifiertypes.ErrRPC,
		verifiertypes.ErrContext,
		verifiertypes.ErrUnknown,
		fetcher.ErrHTTPFetch,
		verifier.ErrActionResultNotFound,
	}
	check := func(t *testing.T, sentinel error, wantStatus string) {
		t.Helper()
		status, message := classifyVerifyStatus(fmt.Errorf("context: %w", sentinel))
		require.Equal(t, wantStatus, status, "wrong verdict for %v", sentinel)
		require.NotEqual(t, "unexpected error", message,
			"%v falls to the default — classifyVerifyStatus is missing a case (drifted from classifyVerifyError)", sentinel)
	}
	for _, s := range rejected {
		check(t, s, types.StatusRejected)
	}
	for _, s := range retry {
		check(t, s, types.StatusRetry)
	}
}

func TestClassifyVerifyStatusDistinguishesRetryReasons(t *testing.T) {
	// An unreachable store and a reachable store that returns unusable data are
	// both RETRY, but must carry distinct messages so operators can tell them apart.
	_, unreachable := classifyVerifyStatus(fmt.Errorf("conn refused: %w", db.ErrDatabase))
	_, unusable := classifyVerifyStatus(fmt.Errorf("bad bytes: %w (boom)", db.ErrDataSource))
	require.NotEqual(t, unreachable, unusable)
	require.Equal(t, "database unavailable", unreachable)
	require.Equal(t, "data source returned unusable data", unusable)
}

func TestVerifyResponseHelpers(t *testing.T) {
	err := errors.New("internal detail that must not leak")

	t.Run("rejectedResponse", func(t *testing.T) {
		resp := rejectedResponse("req1", "log message", "safe reason", err)
		require.Equal(t, types.StatusRejected, resp.Body.Status)
		require.Equal(t, "safe reason", resp.Body.Message)
		require.Empty(t, resp.Body.ResponseBody)
		require.NotContains(t, resp.Body.Message, err.Error())
	})
	t.Run("retryResponse", func(t *testing.T) {
		resp := retryResponse("req2", "log message", "safe reason", err)
		require.Equal(t, types.StatusRetry, resp.Body.Status)
		require.Equal(t, "safe reason", resp.Body.Message)
		require.Empty(t, resp.Body.ResponseBody)
		require.NotContains(t, resp.Body.Message, err.Error())
	})
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

func TestGetVerifierOperationIDUnique(t *testing.T) {
	// A per-source deployment registers these endpoints once per attestation type
	// it serves; the operation IDs must all be distinct or the OpenAPI document is
	// invalid (duplicate operationIds break Swagger/client generation).
	endpoints := []string{"prepareRequestBody", "prepareResponseBody", "verify"}
	types := config.SourceAttestationTypes[config.SourceXRP]
	require.NotEmpty(t, types)

	seen := map[string]bool{}
	for _, at := range types {
		for _, ep := range endpoints {
			id := getVerifierOperationID(config.SourceXRP, at, ep)
			require.Falsef(t, seen[id], "duplicate operation ID: %s", id)
			seen[id] = true
		}
	}
	require.Len(t, seen, len(types)*len(endpoints))
}
