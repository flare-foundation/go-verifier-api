package btcverifier

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/stretchr/testify/require"

	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

func cspABI(t *testing.T) abi.ABI {
	t.Helper()
	a, err := abi.JSON(strings.NewReader(paymentBatchedABI))
	require.NoError(t, err)
	return a
}

// A PaymentBatched log carries the same struct the diamond path carries, on its
// own rather than inside an instruction envelope.
func encodePaymentBatched(t *testing.T, paymentID uint64) *types.Log {
	t.Helper()
	// batchPaymentID and nonce are fixed for the fixture; only paymentID varies per row.
	const batchPaymentID, nonce = 7, 3
	addr, _ := recipient(t)
	msgBytes, err := structs.Encode(utxoArg(t), sampleMsg(addr, 1000, paymentID, batchPaymentID, nonce, 0))
	require.NoError(t, err)

	data, err := cspABI(t).Events["PaymentBatched"].Inputs.NonIndexed().Pack(msgBytes)
	require.NoError(t, err)
	// PaymentBatched indexes (instructionId, paymentId): topic[0]=signature,
	// topic[1]=instructionId, topic[2]=paymentId. The decoder binds the message's
	// paymentId to topic[2], so the fixture must carry it.
	return &types.Log{
		Topics: []common.Hash{
			{}, // [0] event signature (unchecked)
			{}, // [1] instruction id (unchecked here)
			common.BigToHash(new(big.Int).SetUint64(paymentID)),
		},
		Data: data,
	}
}

func TestPaymentBatchedMessagesDecodesEveryPaymentInTheBatch(t *testing.T) {
	src := PaymentBatchedMessages{
		Repo: stubRepo{logs: []*types.Log{
			encodePaymentBatched(t, 7),
			encodePaymentBatched(t, 8),
		}},
		ABI: cspABI(t),
	}

	msgs, err := src.Messages(context.Background(), common.Hash{}, common.Hash{})
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	// Each names its own payment; all share the batch they settled in.
	require.Equal(t, uint64(7), msgs[0].PaymentId)
	require.Equal(t, uint64(8), msgs[1].PaymentId)
	for _, m := range msgs {
		require.Equal(t, uint64(7), m.BatchPaymentId)
		require.Equal(t, uint64(3), m.Nonce)
	}
}

// An ABI without the event is a misconfiguration, not a missing payment: it must
// not read as "this payment was never instructed".
func TestPaymentBatchedMessagesRejectsAnABIWithoutTheEvent(t *testing.T) {
	src := PaymentBatchedMessages{Repo: stubRepo{}, ABI: abi.ABI{}}
	_, err := src.Messages(context.Background(), common.Hash{}, common.Hash{})
	require.ErrorContains(t, err, "PaymentBatched")
}

// TestPaymentBatchedMessagesRejectsPaymentIdTopicMismatch: the decoded message's
// paymentId must equal the indexed topic; a disagreement is a corrupt row.
func TestPaymentBatchedMessagesRejectsPaymentIdTopicMismatch(t *testing.T) {
	log := encodePaymentBatched(t, 7)                           // body says paymentId 7
	log.Topics[2] = common.BigToHash(new(big.Int).SetUint64(8)) // topic says 8
	src := PaymentBatchedMessages{Repo: stubRepo{logs: []*types.Log{log}}, ABI: cspABI(t)}
	_, err := src.Messages(context.Background(), common.Hash{}, common.Hash{})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

// cloneLog deep-copies a log so a table case can mutate its topics safely.
func cloneLog(l *types.Log) *types.Log {
	c := *l
	c.Topics = append([]common.Hash(nil), l.Topics...)
	c.Data = append([]byte(nil), l.Data...)
	return &c
}

// TestPaymentBatchedMessages_HostileLogs: every malformed/hostile row is refused
// (fail closed), never decoded into a message.
func TestPaymentBatchedMessages_HostileLogs(t *testing.T) {
	valid := encodePaymentBatched(t, 7)
	overlongEnvelope := func() *types.Log {
		l := cloneLog(valid)
		data, err := cspABI(t).Events["PaymentBatched"].Inputs.NonIndexed().Pack([]byte{0xde, 0xad}) // valid envelope, garbage body
		require.NoError(t, err)
		l.Data = data
		return l
	}()
	oversizedTopic := cloneLog(valid)
	oversizedTopic.Topics[2] = common.HexToHash(strings.Repeat("f", 64)) // 2^256-1, not a uint64
	tooManyBytes := cloneLog(valid)
	tooManyBytes.Data = make([]byte, (1<<20)+1)
	fewTopics := cloneLog(valid)
	fewTopics.Topics = fewTopics.Topics[:2]

	cases := []struct {
		name string
		log  *types.Log
	}{
		{"fewer than three topics", fewTopics},
		{"payment topic exceeds uint64", oversizedTopic},
		{"malformed ABI envelope", &types.Log{Topics: valid.Topics, Data: []byte{0x01, 0x02}}},
		{"malformed encoded message", overlongEnvelope},
		{"data larger than 1 MiB", tooManyBytes},
		{"nil log", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := PaymentBatchedMessages{Repo: stubRepo{logs: []*types.Log{tc.log}}, ABI: cspABI(t)}
			_, err := src.Messages(context.Background(), common.Hash{}, common.Hash{})
			require.ErrorIs(t, err, paymentdb.ErrDatabase)
		})
	}
}
