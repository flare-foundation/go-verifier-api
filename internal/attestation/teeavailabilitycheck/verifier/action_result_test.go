package verifier

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

// signActionResult signs result.Hash() with the given key using the same
// eth-personal-sign scheme that utils.VerifySignature expects.
func signActionResult(t *testing.T, result teenodetypes.ActionResult, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	hash := result.Hash()
	ethHash := accounts.TextHash(hash)
	sig, err := crypto.Sign(ethHash, key)
	require.NoError(t, err)
	return sig
}

func TestVerifyActionResult(t *testing.T) {
	teeKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	teeID := crypto.PubkeyToAddress(teeKey.PublicKey)

	instructionID := common.HexToHash("0xdeadbeef")

	validResult := teenodetypes.ActionResult{
		ID:     instructionID,
		Status: 1, // success for direct instructions (see tee-node direct/direct.go)
		Data:   []byte(`{"teeInfo":{}}`),
	}

	t.Run("valid action result", func(t *testing.T) {
		sig := signActionResult(t, validResult, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: sig,
		}
		require.NoError(t, verifyActionResult(resp, instructionID, teeID))
	})

	t.Run("Result.ID mismatch is a replay", func(t *testing.T) {
		// TEE validly signed an action result for a DIFFERENT instruction.
		// Proxy replays it for a request expecting `instructionID`.
		other := validResult
		other.ID = common.HexToHash("0xcafef00d")
		sig := signActionResult(t, other, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    other,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "instruction ID mismatch")
	})

	t.Run("Status 0 (Invalid) is rejected with diagnostics", func(t *testing.T) {
		failed := validResult
		failed.Status = 0 // processorutils.Invalid sets Status=0 with Log populated
		failed.Log = "invalid instruction"
		failed.AdditionalResultStatus = []byte{0xab, 0xcd}
		sig := signActionResult(t, failed, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    failed,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "status not success")
		require.Contains(t, err.Error(), "status=0")
		require.Contains(t, err.Error(), "invalid instruction")
		require.Contains(t, err.Error(), "abcd")
	})

	t.Run("Status 3 (DeadlineExceeded) is rejected", func(t *testing.T) {
		failed := validResult
		failed.Status = 3
		failed.Log = "deadline exceeded"
		sig := signActionResult(t, failed, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    failed,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "status not success")
		require.Contains(t, err.Error(), "status=3")
	})

	t.Run("missing TEE signature", func(t *testing.T) {
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: nil,
		}
		err := verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing TEE signature")
	})

	t.Run("signature from a different TEE is rejected", func(t *testing.T) {
		// TEE A signs the action result, but the request expected TEE B.
		otherKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		sig := signActionResult(t, validResult, otherKey)
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: sig,
		}
		err = verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "TEE signature on action result does not match expected TEE")
	})

	t.Run("malformed signature is rejected", func(t *testing.T) {
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: []byte("not-a-real-signature"),
		}
		err := verifyActionResult(resp, instructionID, teeID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "TEE signature on action result")
	})
}
