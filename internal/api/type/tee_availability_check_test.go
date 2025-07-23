package attestationtypes_test

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/tee"
	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
)

func TestTeeInfoHash(t *testing.T) {
	var x [32]byte
	var y [32]byte
	for i := 0; i < 32; i++ {
		x[i] = byte(i)
		y[i] = byte(32 - i)
	}

	mockPublicKey := tee.PublicKey{
		X: x,
		Y: y,
	}
	mockData := apitypes.ProxyInfoData{
		Challenge:                common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000001234"),
		PublicKey:                mockPublicKey,
		InitialSigningPolicyId:   1,
		InitialSigningPolicyHash: common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000abcd"),
		LastSigningPolicyId:      2,
		LastSigningPolicyHash:    common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000dead"),
		StateHash:                common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000beef"),
		TeeTimestamp:             123456789,
	}
	hash, err := mockData.TeeInfoHash()
	if err != nil {
		t.Fatalf("TeeInfoHash returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("Expected non-empty hash string")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		t.Fatalf("Hash is not valid hex: %v", err)
	}
	t.Logf("Generated TeeInfoHash: %s", hash)
}
