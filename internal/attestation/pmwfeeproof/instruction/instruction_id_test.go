package instruction_test

import (
	"encoding/binary"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// rightPad32 and encodeInstructionIDLikeSolidity independently reproduce the
// contract's _computeInstructionId (TeePaymentsBase.sol):
//
//	keccak256(abi.encode(bytes32 opType, bytes32 opCommand, bytes32 sourceId,
//	                     string accountAddress, uint64 paymentId, uint64 reissueNumber))
//
// They lay out the ABI head/tail bytes by hand and share no code with the
// production packer, so drift in argument order, types, packed-vs-encode, the
// string offset, or the PAY/REISSUE op constants makes them disagree. (Duplicated
// from the PMWPaymentStatus pin test — the two instruction packages are
// independent.)
func rightPad32(s string) [32]byte {
	var b [32]byte
	copy(b[:], s)
	return b
}

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

// TestInstructionIDsPinnedToContract confirms, byte-for-byte against an
// independent reconstruction of TeePaymentsBase._computeInstructionId, that both
// the PAY (reissueNumber 0) and REISSUE (reissueNumber >= 1) instruction IDs match
// the contract/API — the encodings differ only by opCommand and reissueNumber.
func TestInstructionIDsPinnedToContract(t *testing.T) {
	account := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentID := uint64(11263155)
	opType := rightPad32("F_XRP") // op.XRP — the XRP source op type
	sourceID := rightPad32("testXRP")

	// PAY: reissueNumber is always 0.
	payWant := encodeInstructionIDLikeSolidity(opType, rightPad32("PAY"), sourceID, account, paymentID, 0)
	require.Equal(t, "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09", payWant.Hex())
	payGot, err := instruction.GeneratePayInstructionID(opType, sourceID, account, paymentID)
	require.NoError(t, err)
	require.Equal(t, payWant, payGot)

	// REISSUE: reissues start at 1.
	reissueWant := encodeInstructionIDLikeSolidity(opType, rightPad32("REISSUE"), sourceID, account, paymentID, 1)
	require.Equal(t, "0x692a7a505622d10e9727201d8fdd0562188fdad19b4c6a45436a740a180109f5", reissueWant.Hex())
	reissueGot, err := instruction.GenerateReissueInstructionID(opType, sourceID, account, paymentID, 1)
	require.NoError(t, err)
	require.Equal(t, reissueWant, reissueGot)
}

func TestGeneratePayInstructionID(t *testing.T) {
	// Same expected value as PMWPaymentStatus GenerateInstructionID — identical encoding.
	expected := "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	require.Equal(t, expected, id.Hex())
}

func TestGenerateReissueInstructionID(t *testing.T) {
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)

	// Reissue ID must differ from pay ID for the same paymentId.
	payID, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	reissueID, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId, 1)
	require.NoError(t, err)
	require.NotEqual(t, payID, reissueID)

	// Different reissueNumbers produce different IDs.
	reissueID1, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId, 2)
	require.NoError(t, err)
	require.NotEqual(t, reissueID, reissueID1)
}
