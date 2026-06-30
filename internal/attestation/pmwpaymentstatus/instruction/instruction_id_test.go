package instruction_test

import (
	"encoding/binary"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// rightPad32 reproduces the canonical string→bytes32 encoding (ASCII, right-padded
// with zeros) the off-chain caller uses for opType/sourceId/opCommand. Hardcoding
// it here (rather than calling convert.StringToCommonHash) lets the pin test catch
// drift in that helper or in the PAY/REISSUE op constants.
func rightPad32(s string) [32]byte {
	var b [32]byte
	copy(b[:], s)
	return b
}

// encodeInstructionIDLikeSolidity independently reproduces the contract's
// _computeInstructionId (TeePaymentsBase.sol):
//
//	keccak256(abi.encode(bytes32 opType, bytes32 opCommand, bytes32 sourceId,
//	                     string accountAddress, uint64 paymentId, uint64 reissueNumber))
//
// It lays out the ABI head/tail bytes by hand and shares no code with the
// production packer, so any drift in argument order, types, packed-vs-encode, or
// the string offset makes this disagree with the production output.
func encodeInstructionIDLikeSolidity(opType, opCommand, sourceID [32]byte, account string, paymentID, reissueNumber uint64) common.Hash {
	word := func(n uint64) []byte {
		w := make([]byte, 32)
		binary.BigEndian.PutUint64(w[24:], n)
		return w
	}
	const headWords = 6 // six top-level args; the string is encoded as an offset into the tail
	var buf []byte
	buf = append(buf, opType[:]...)                  // head[0]
	buf = append(buf, opCommand[:]...)               // head[1]
	buf = append(buf, sourceID[:]...)                // head[2]
	buf = append(buf, word(headWords*32)...)         // head[3] = offset to string tail (192)
	buf = append(buf, word(paymentID)...)            // head[4]
	buf = append(buf, word(reissueNumber)...)        // head[5]
	buf = append(buf, word(uint64(len(account)))...) // tail: string length
	data := make([]byte, (len(account)+31)/32*32)    // tail: string bytes, right-padded to a word
	copy(data, account)
	buf = append(buf, data...)
	return crypto.Keccak256Hash(buf)
}

// TestGenerateInstructionIDPinnedToContract confirms, byte-for-byte against an
// independent reconstruction of TeePaymentsBase._computeInstructionId, that the
// PAY instruction ID encoding has not drifted from the contract/API. Cheap
// insurance for the whole PMW path.
func TestGenerateInstructionIDPinnedToContract(t *testing.T) {
	account := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentID := uint64(11263155)
	opType := rightPad32("F_XRP") // op.XRP — the XRP source op type
	sourceID := rightPad32("testXRP")
	payCmd := rightPad32("PAY") // PAY events always use reissueNumber 0

	// Independent reconstruction of the contract's abi.encode(...) + keccak256.
	want := encodeInstructionIDLikeSolidity(opType, payCmd, sourceID, account, paymentID, 0)

	// The committed golden must equal that independent value (re-confirms it is the
	// contract-correct hash, not merely self-consistent with the production code).
	require.Equal(t, "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09", want.Hex())

	// Production output must match the independent reconstruction byte-for-byte.
	got, err := instruction.GenerateInstructionID(opType, sourceID, account, paymentID)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestGenerateInstructionID(t *testing.T) {
	expected := "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, expected, id.Hex())
}
