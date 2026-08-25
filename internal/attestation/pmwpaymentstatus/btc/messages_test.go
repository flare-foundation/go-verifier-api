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
func encodePaymentBatched(t *testing.T, paymentID, batchPaymentID, nonce uint64) *types.Log {
	t.Helper()
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
			encodePaymentBatched(t, 7, 7, 3),
			encodePaymentBatched(t, 8, 7, 3),
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
	log := encodePaymentBatched(t, 7, 7, 3)                     // body says paymentId 7
	log.Topics[2] = common.BigToHash(new(big.Int).SetUint64(8)) // topic says 8
	src := PaymentBatchedMessages{Repo: stubRepo{logs: []*types.Log{log}}, ABI: cspABI(t)}
	_, err := src.Messages(context.Background(), common.Hash{}, common.Hash{})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}
