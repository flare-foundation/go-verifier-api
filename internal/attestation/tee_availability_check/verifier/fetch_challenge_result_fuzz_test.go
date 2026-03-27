package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
)

func FuzzFetchTEEChallengeResult(f *testing.F) {
	challengeID := common.HexToHash("0x123")
	validJSON := []byte(`{"teeInfo":{"InitialSigningPolicyID":1}}`)

	privKey, err := crypto.GenerateKey()
	if err != nil {
		f.Fatal(err)
	}
	hash := crypto.Keccak256(validJSON)
	ethHash := accounts.TextHash(hash)
	validSignature, err := crypto.Sign(ethHash, privKey)
	if err != nil {
		f.Fatal(err)
	}

	type fuzzResponse struct {
		mu   sync.RWMutex
		data []byte
		sig  []byte
	}

	state := &fuzzResponse{data: append([]byte(nil), validJSON...), sig: append([]byte(nil), validSignature...)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.RLock()
		resp := teenodetypes.ActionResponse{
			Result:         teenodetypes.ActionResult{Data: hexutil.Bytes(append([]byte(nil), state.data...))},
			ProxySignature: append([]byte(nil), state.sig...),
		}
		state.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	f.Cleanup(server.Close)

	f.Add(validJSON, validSignature)
	f.Add([]byte("not-json"), validSignature)
	f.Add([]byte{}, validSignature)
	f.Add([]byte(`{"teeInfo":"not-an-object"}`), validSignature)
	f.Add(validJSON, []byte("invalid-signature"))
	f.Add([]byte(`null`), validSignature)

	f.Fuzz(func(t *testing.T, data []byte, signature []byte) {
		state.mu.Lock()
		state.data = append(state.data[:0], data...)
		state.sig = append(state.sig[:0], signature...)
		state.mu.Unlock()

		teeInfo, signer, err := FetchTEEChallengeResult(context.Background(), server.URL, challengeID, true)
		if err != nil {
			if !reflect.DeepEqual(teeInfo, teenodetypes.TeeInfoResponse{}) {
				t.Fatal("failed fetch returned non-zero tee info")
			}
			if signer != (common.Address{}) {
				t.Fatal("failed fetch returned non-zero signer")
			}
			return
		}

		if len(data) == 0 {
			t.Fatal("empty challenge result data unexpectedly succeeded")
		}
		if !json.Valid(data) {
			t.Fatal("invalid JSON challenge result unexpectedly succeeded")
		}
		if signer == (common.Address{}) {
			t.Fatal("successful fetch returned zero signer")
		}

		var expected teenodetypes.TeeInfoResponse
		if err := json.Unmarshal(data, &expected); err != nil {
			t.Fatalf("successful fetch returned data that cannot be unmarshaled locally: %v", err)
		}
		if !reflect.DeepEqual(expected, teeInfo) {
			t.Fatalf("successful fetch returned unexpected tee info: got %+v want %+v", teeInfo, expected)
		}
	})
}
