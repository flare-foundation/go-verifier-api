package utils

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

var (
	ErrNotFound = errors.New("resource not found (404)")
)

func Bytes32(s string) ([32]byte, error) {
	var b [32]byte
	if len(s) > 32 {
		return b, fmt.Errorf("string %s too long for Bytes32", s)
	}
	copy(b[:], s)
	return b, nil
}

func AbiEncodeRequestData[T any](data T, arg abi.Argument) ([]byte, error) {
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request data': %v", err)
	}
	structs.Encode(connector.AttestationRequestArg, &connector.IFtdcHubFtdcAttestationRequest{})

	return encoded, nil
}

func AbiDecodeRequestData[T any](data []byte, arg abi.Argument) (T, error) {
	decode, err := structs.Decode[T](arg, data)
	if err != nil {
		var zero T
		return zero, err
	}
	return decode, nil
}

func AbiEncodeResponseData[T any](data T, arg abi.Argument) ([]byte, error) {
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode response data: %v", err)
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
		return zero, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return zero, ErrNotFound
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

func HexWith0x(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}
