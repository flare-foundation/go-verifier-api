package btcverifier

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// FuzzPaymentBatchedMessages: arbitrary event-data bytes must never panic, and any
// error result must carry no decoded messages (fail closed). The 1 MiB guard keeps
// runtime/allocation bounded.
func FuzzPaymentBatchedMessages(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x01, 0x02})
	f.Add(bytes.Repeat([]byte{0xff}, 64))
	f.Fuzz(func(t *testing.T, data []byte) {
		log := &types.Log{
			Topics: []common.Hash{{}, {}, common.BigToHash(big.NewInt(7))},
			Data:   data,
		}
		src := PaymentBatchedMessages{Repo: stubRepo{logs: []*types.Log{log}}, ABI: cspABI(t)}
		msgs, err := src.Messages(context.Background(), common.Hash{}, common.Hash{}) // must not panic
		if err != nil && msgs != nil {
			t.Fatalf("error result must return no messages, got %d", len(msgs))
		}
	})
}
