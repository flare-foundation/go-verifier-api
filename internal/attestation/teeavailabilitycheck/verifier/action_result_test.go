package verifier

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	csigning "github.com/flare-foundation/go-flare-common/pkg/signing"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

// testChainID is bound into the action-result signing preimage in these tests
// and must match the chainID passed to verifyActionResult.
const testChainID uint64 = 14

// signActionResult signs the domain-separated action-result preimage
// DomainHash(TEE_ACTION_RESULT, testChainID, result.Hash()) with the given key
// using the eth-personal-sign scheme that utils.VerifySignature expects.
func signActionResult(t *testing.T, result teenodetypes.ActionResult, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	signHash, err := csigning.NewPayload(csigning.TEEActionResult, testChainID, common.BytesToHash(result.Hash())).Hash()
	require.NoError(t, err)
	ethHash := accounts.TextHash(signHash[:])
	sig, err := crypto.Sign(ethHash, key)
	require.NoError(t, err)
	return sig
}

func TestVerifyActionResult(t *testing.T) {
	teeKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	teeID := crypto.PubkeyToAddress(teeKey.PublicKey)

	instructionID := common.HexToHash("0xdeadbeef")

	// (op.Reg, op.TEEAttestation) is the only pair that reaches the verifier
	// via the FDC2 availability-check flow (the embedded instructionId in the
	// request body always points to a prior admission action result).
	validResult := teenodetypes.ActionResult{
		ID:        instructionID,
		Status:    1, // regutils.TEEAttestation sets Status=1 on success
		OPType:    op.Reg.Hash(),
		OPCommand: op.TEEAttestation.Hash(),
		Data:      []byte(`{"teeInfo":{}}`),
	}

	t.Run("valid (Reg, TEEAttestation) — admission action result", func(t *testing.T) {
		sig := signActionResult(t, validResult, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: sig,
		}
		require.NoError(t, verifyActionResult(resp, instructionID, teeID, testChainID))
	})

	t.Run("(Get, TEEInfo) is rejected — not reachable via FDC2 flow", func(t *testing.T) {
		// tee-node's getutils.TEEInfo direct processor exists but is not on
		// the trusted attestation path. A proxy returning it for an
		// availability-check verify is suspect.
		mismatched := validResult
		mismatched.OPType = op.Get.Hash()
		mismatched.OPCommand = op.TEEInfo.Hash()
		sig := signActionResult(t, mismatched, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    mismatched,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "OPType mismatch")
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
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
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
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
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
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "status not success")
		require.Contains(t, err.Error(), "status=3")
	})

	t.Run("OPType mismatch is rejected", func(t *testing.T) {
		// OPType is not bound by Result.Hash(), so a malicious proxy can
		// tamper with it without breaking the TEE signature.
		mismatched := validResult
		mismatched.OPType = op.XRP.Hash() // payment op, not an availability response
		sig := signActionResult(t, mismatched, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    mismatched,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "OPType mismatch")
	})

	t.Run("OPCommand mismatch is rejected", func(t *testing.T) {
		// OPType matches but OPCommand doesn't (e.g. Reg / KeyGenerate
		// instead of Reg / TEEAttestation).
		mismatched := validResult
		mismatched.OPCommand = op.KeyGenerate.Hash()
		sig := signActionResult(t, mismatched, teeKey)
		resp := teenodetypes.ActionResponse{
			Result:    mismatched,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "OPCommand mismatch")
	})

	t.Run("missing TEE signature", func(t *testing.T) {
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: nil,
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
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
		err = verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "TEE signature on action result does not match expected TEE")
	})

	t.Run("malformed signature is rejected", func(t *testing.T) {
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: []byte("not-a-real-signature"),
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "TEE signature on action result")
	})

	t.Run("signature bound to a different chainID is rejected", func(t *testing.T) {
		// The TEE signs over DomainHash(TEE_ACTION_RESULT, testChainID, …);
		// verifying against a different chainID changes the preimage, so signer
		// recovery yields an address other than the expected TEE. This is the
		// cross-chain replay protection the chainID binding provides.
		sig := signActionResult(t, validResult, teeKey) // signed with testChainID
		resp := teenodetypes.ActionResponse{
			Result:    validResult,
			Signature: sig,
		}
		err := verifyActionResult(resp, instructionID, teeID, testChainID+1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "TEE signature on action result does not match expected TEE")
	})
}
