package btcverifier

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/stretchr/testify/require"
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
	return &types.Log{Data: data}
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
