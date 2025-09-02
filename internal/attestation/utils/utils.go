package utils

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"
)

var (
	ErrNotFound     = errors.New("resource not found (404)")
	ErrNetwork      = errors.New("network error")
	ErrRPC          = errors.New("rpc error")
	ErrInvalidInput = errors.New("invalid input")
	ErrContext      = errors.New("context error")
	ErrUnknown      = errors.New("unknown error")
)

const (
	Bytes32Size = 32
)

func Bytes32(s string) ([32]byte, error) {
	var b [32]byte
	if len(s) > Bytes32Size {
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
	if len(b) != Bytes32Size {
		return arr, fmt.Errorf("invalid length for bytes32: got %d bytes, expected 32", len(b))
	}
	copy(arr[:], b)
	return arr, nil
}

func NewBigIntFromString(s string) (*big.Int, error) {
	const decimalBase = 10
	i, ok := new(big.Int).SetString(s, decimalBase)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	return i, nil
}

func ClassifyFetchError(op string, err error) (teetypes.TeePollerSampleState, error) {
	wrapErr := func(e error) error {
		return &FetchError{Op: op, Err: e}
	}
	// Context issues
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrContext)
	}
	// HTTP layer (non-200 responses)
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound:
			return teetypes.TeePollerSampleInvalid, wrapErr(ErrInvalidInput)
		default:
			return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrNetwork)
		}
	}
	// JSON-RPC structured errors
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		switch rpcErr.ErrorCode() {
		// Deterministic client-side issues → invalid
		case -32600, // invalid request
			-32601, // method not found
			-32602, // invalid params
			-32700: // parse error
			return teetypes.TeePollerSampleInvalid, wrapErr(ErrInvalidInput)
		// Infra/server-side issues → indeterminate
		case -32000, // generic server error,
			-32002, // timeout
			-32003, // response too large
			-32603: // internal error
			return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrRPC)
		default:
			return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrRPC)
		}
	}
	// Network issues (DNS fail, conn refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrNetwork)
	}

	return teetypes.TeePollerSampleIndeterminate, wrapErr(ErrUnknown)
}

type FetchError struct {
	Op  string
	Err error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}
