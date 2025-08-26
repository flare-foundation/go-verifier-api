package utils

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
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

func AbiEncodeData[T any](data T, arg abi.Argument) ([]byte, error) {
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, err
	}
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

func AbiDecodeEventData[T any](abiObj abi.ABI, eventName string, data []byte) (*T, error) {
	var result T
	err := abiObj.UnpackIntoInterface(&result, eventName, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode event %s: %w", eventName, err)
	}
	return &result, nil
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

func BytesToHex0x(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}

func RemoveHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

func HexStringToBytes32(s string) (common.Hash, error) {
	var arr common.Hash
	s = RemoveHexPrefix(s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return arr, err
	}
	if len(b) != 32 {
		return arr, fmt.Errorf("invalid length for bytes32: got %d bytes, expected 32", len(b))
	}
	copy(arr[:], b)
	return arr, nil
}

func NewBigIntFromString(s string) (*big.Int, error) {
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	return i, nil
}
