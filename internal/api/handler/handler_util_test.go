package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/go-chi/chi/v5"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
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
	encodedAndABI := loadTestEncodedAndABI(t)
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
	encodedAndABI := loadTestEncodedAndABI(t)
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
	encodedAndABI := loadTestEncodedAndABI(t)
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
	encodedAndABI := loadTestEncodedAndABI(t)
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
	encodedAndABI := loadTestEncodedAndABI(t)
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

func loadTestEncodedAndABI(t *testing.T) *config.EncodedAndABI {
	t.Helper()
	attestationType := connector.PMWMultisigAccountConfigured
	encodedAndABI, err := config.LoadEncodedAndABI(config.EnvConfig{
		APIKeys:         nil,
		AttestationType: attestationType,
		SourceID:        config.SourceTestXRP,
	})
	require.NoError(t, err)
	return &encodedAndABI
}

func TestPublishSnapshot(t *testing.T) {
	t.Run("empty samples", func(t *testing.T) {
		v := &verifier.TeeVerifier{
			TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
		}
		v.PublishSnapshot()
		snap := v.PollerSnapshot.Load().([]verifiertypes.TeeSample) //nolint:forcetypeassert // test-only, type guaranteed by PublishSnapshot
		require.Empty(t, snap)
	})

	t.Run("with samples", func(t *testing.T) {
		teeAddr := common.HexToAddress("0x1234")
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeAddr: {
					{State: verifiertypes.TeeSampleValid},
					{State: verifiertypes.TeeSampleInvalid},
				},
			},
		}
		v.PublishSnapshot()
		snap := v.PollerSnapshot.Load().([]verifiertypes.TeeSample) //nolint:forcetypeassert // test-only, type guaranteed by PublishSnapshot
		require.Len(t, snap, 1)
		require.Equal(t, teeAddr.Hex(), snap[0].TeeID)
		require.Len(t, snap[0].Values, 2)
	})

	t.Run("snapshot is sorted by TeeID", func(t *testing.T) {
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				common.HexToAddress("0xBBBB"): {{State: verifiertypes.TeeSampleValid}},
				common.HexToAddress("0xAAAA"): {{State: verifiertypes.TeeSampleValid}},
				common.HexToAddress("0xCCCC"): {{State: verifiertypes.TeeSampleValid}},
			},
		}
		v.PublishSnapshot()
		snap := v.PollerSnapshot.Load().([]verifiertypes.TeeSample) //nolint:forcetypeassert // test-only, type guaranteed by PublishSnapshot
		require.Len(t, snap, 3)
		require.True(t, snap[0].TeeID < snap[1].TeeID)
		require.True(t, snap[1].TeeID < snap[2].TeeID)
	})

	t.Run("snapshot is decoupled from internal storage", func(t *testing.T) {
		teeAddr := common.HexToAddress("0xabcd")
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeAddr: {{State: verifiertypes.TeeSampleValid}},
			},
		}
		v.PublishSnapshot()
		snap := v.PollerSnapshot.Load().([]verifiertypes.TeeSample) //nolint:forcetypeassert // test-only, type guaranteed by PublishSnapshot
		require.Len(t, snap, 1)
		snap[0].Values[0].State = verifiertypes.TeeSampleInvalid

		v.SamplesMu.RLock()
		defer v.SamplesMu.RUnlock()
		require.Equal(t, verifiertypes.TeeSampleValid, v.TeeSamples[teeAddr][0].State)
	})

	t.Run("concurrent publish and read do not race", func(t *testing.T) {
		teeAddr := common.HexToAddress("0xdead")
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeAddr: {{State: verifiertypes.TeeSampleValid}},
			},
		}
		v.PublishSnapshot()
		stop := make(chan struct{})
		var wg sync.WaitGroup

		// Writer: simulate the poller appending samples + publishing.
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					v.SamplesMu.Lock()
					v.TeeSamples[teeAddr] = append(v.TeeSamples[teeAddr], verifiertypes.TeeSampleValue{State: verifiertypes.TeeSampleValid})
					v.SamplesMu.Unlock()
					v.PublishSnapshot()
				}
			}
		})

		// Readers: many concurrent atomic loads (simulating /poller/tees endpoint).
		for range 10 {
			wg.Go(func() {
				for range 100 {
					_ = v.PollerSnapshot.Load()
				}
			})
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			close(stop)
		}()
		wg.Wait()
	})
}

func TestPollerTeesEndpointPagination(t *testing.T) {
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("test", "1.0"))

	v := &verifier.TeeVerifier{
		TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
	}
	// Populate 5 TEEs.
	for i := range 5 {
		addr := common.BigToAddress(big.NewInt(int64(i + 1)))
		v.TeeSamples[addr] = []verifiertypes.TeeSampleValue{
			{Timestamp: time.Now(), State: verifiertypes.TeeSampleValid},
		}
	}
	v.PublishSnapshot()
	RegisterTeePoolingHandler(api, v)

	type samplesResponse struct {
		Samples []struct {
			TeeID  string `json:"tee_id"`
			Values []struct {
				State string `json:"state"`
			} `json:"values"`
		} `json:"samples"`
		Total int `json:"total"`
	}
	get := func(t *testing.T, query string) samplesResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/poller/tees"+query, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp samplesResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp
	}

	t.Run("default returns all when under limit", func(t *testing.T) {
		resp := get(t, "")
		require.Len(t, resp.Samples, 5)
		require.Equal(t, 5, resp.Total)
	})

	t.Run("limit restricts page size", func(t *testing.T) {
		resp := get(t, "?limit=2")
		require.Len(t, resp.Samples, 2)
		require.Equal(t, 5, resp.Total)
	})

	t.Run("offset skips entries", func(t *testing.T) {
		resp := get(t, "?offset=3&limit=10")
		require.Len(t, resp.Samples, 2)
		require.Equal(t, 5, resp.Total)
	})

	t.Run("offset beyond total returns empty", func(t *testing.T) {
		resp := get(t, "?offset=100")
		require.Empty(t, resp.Samples)
		require.Equal(t, 5, resp.Total)
	})

	t.Run("results are sorted by TeeID", func(t *testing.T) {
		resp := get(t, "")
		for i := 1; i < len(resp.Samples); i++ {
			require.True(t, resp.Samples[i-1].TeeID < resp.Samples[i].TeeID)
		}
	})

	t.Run("before snapshot published returns empty", func(t *testing.T) {
		emptyV := &verifier.TeeVerifier{
			TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
		}
		emptyRouter := chi.NewMux()
		emptyAPI := humachi.New(emptyRouter, huma.DefaultConfig("test", "1.0"))
		RegisterTeePoolingHandler(emptyAPI, emptyV)

		req := httptest.NewRequest(http.MethodGet, "/poller/tees", nil)
		w := httptest.NewRecorder()
		emptyRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp samplesResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Empty(t, resp.Samples)
		require.Equal(t, 0, resp.Total)
	})
}

func TestClassifyVerifyError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
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
		// 503 — TEE infrastructure errors
		{
			name:           "ErrInsufficientSamples",
			err:            fmt.Errorf("not enough data: %w", verifier.ErrInsufficientSamples),
			expectedStatus: http.StatusServiceUnavailable,
		},
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
