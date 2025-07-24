package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	teeavailabilitycheckconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
)

func Bytes32(s string) [32]byte {
	var b [32]byte
	// if len(s) > 32 { // TODO - need this check?
	// 	return b, fmt.Errorf("string too long for Bytes32")
	// }
	copy(b[:], s)
	return b
}

func AbiEncodeRequestData(data types.TeeAvailabilityRequestData) ([]byte, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeRequestArg()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckRequestBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckRequestBody': %v", err)
	}
	structs.Encode(connector.AttestationRequestArg, &connector.IFtdcHubFtdcAttestationRequest{})

	return encoded, nil
}

func AbiDecodeRequestData(data []byte) (types.TeeAvailabilityRequestData, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeRequestArg()
	if err != nil {
		return types.TeeAvailabilityRequestData{}, fmt.Errorf("failed to get 'TeeAvailabilityCheckRequestBody' ABI argument: %v", err)
	}
	decode, err := structs.Decode[types.TeeAvailabilityRequestData](arg, data)
	if err != nil {
		return types.TeeAvailabilityRequestData{}, err
	}
	return decode, nil
}

func AbiEncodeResponseData(data types.TeeAvailabilityResponseData) ([]byte, error) {
	arg, err := teeavailabilitycheckconfig.GetTeeResponseArg()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'TeeAvailabilityCheckResponseBody' ABI argument: %v", err)
	}
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode 'TeeAvailabilityCheckResponseBody': %v", err)
	}
	return encoded, nil
}

func FetchJSON[T any](ctx context.Context, url string, fetchTimeout time.Duration) (T, error) {
	var zero T
	httpClient := &http.Client{
		Timeout: fetchTimeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, fmt.Errorf("creating HTTP request failed for url %s: %w", url, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("HTTP request failed for url %s: %w", url, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return zero, nil
	case http.StatusOK:
		// proceed
	default:
		return zero, fmt.Errorf("unexpected status code: %d for url %s", resp.StatusCode, url)
	}
	err = json.NewDecoder(resp.Body).Decode(&zero)
	if err != nil {
		return zero, fmt.Errorf("decoding failed for type %s: %w", reflect.TypeOf(zero), err)
	}
	return zero, nil
}
