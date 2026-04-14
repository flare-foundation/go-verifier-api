package xrpverifier

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestVerifyFeeProof(t *testing.T) {
	t.Run("single nonce success", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_single",
			[]uint64{100},
			[]int64{50},    // maxFee
			[]string{"12"}, // txFee
		)
		resp, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.NoError(t, err)
		require.Equal(t, big.NewInt(50), resp.EstimatedFee) // pay maxFee only, no reissues
		require.Equal(t, big.NewInt(12), resp.ActualFee)
	})

	t.Run("multiple nonces sum correctly", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_multi",
			[]uint64{100, 101, 102},
			[]int64{50, 60, 70},        // maxFees
			[]string{"10", "15", "20"}, // txFees
		)
		resp, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        102,
			UntilTimestamp: 1800000000,
		})
		require.NoError(t, err)
		require.Equal(t, big.NewInt(50+60+70), resp.EstimatedFee)
		require.Equal(t, big.NewInt(10+15+20), resp.ActualFee)
	})

	t.Run("missing pay event returns error", func(t *testing.T) {
		// Seed nonces 100 and 101, but request 100-102.
		f := setupFeeProofFixture(t, "fp_missing_pay",
			[]uint64{100, 101},
			[]int64{50, 60},
			[]string{"10", "15"},
		)
		_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        102, // nonce 102 has no pay event
			UntilTimestamp: 1800000000,
		})
		require.ErrorIs(t, err, ErrMissingPayEvent)
	})

	t.Run("missing transaction returns error", func(t *testing.T) {
		teeABI := testTeeABI(t)
		xrpDB := testSharedDB(t, "fp_notx_xrp", &paymentdb.DBTransaction{})
		cChainDB := testSharedDB(t, "fp_notx_cchain", &database.Log{})

		sourceID := common.HexToHash("0x1")
		opType := common.HexToHash("0xAA")
		nonce := uint64(100)

		// Seed only the pay event, no transaction.
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, "rSender", nonce)
		require.NoError(t, err)
		eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
		require.NoError(t, err)

		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         nonce,
			SubNonce:      nonce,
		}
		eventData := testEncodeEvent(t, teeABI, op.Pay, msg)

		require.NoError(t, cChainDB.Create(&database.Log{
			Topic0:          trimHex(eventHash),
			Topic1:          trimHex(common.HexToHash("").Hex()),
			Topic2:          trimHex(payID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
			Timestamp:       1700000000,
			BlockNumber:     100,
		}).Error)

		cfg := &config.PMWFeeProofConfig{
			ParsedTeeInstructionsABI: teeABI,
			EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
		}
		v := &XRPVerifier{
			Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: cfg,
		}

		_, err = v.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:         opType,
			SenderAddress:  "rSender",
			FromNonce:      nonce,
			ToNonce:        nonce,
			UntilTimestamp: 1800000000,
		})
		require.ErrorIs(t, err, ErrMissingTransaction)
	})

	t.Run("toNonce < fromNonce returns error", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_badrange", []uint64{100}, []int64{50}, []string{"10"})
		_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:        f.opType,
			SenderAddress: "rSender",
			FromNonce:     10,
			ToNonce:       5,
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
	})

	t.Run("range exceeds max returns error", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_bigrange", []uint64{100}, []int64{50}, []string{"10"})
		_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:        f.opType,
			SenderAddress: "rSender",
			FromNonce:     1,
			ToNonce:       1 + MaxNonceRange,
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
	})

	t.Run("malformed tx fee returns error", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_badfee",
			[]uint64{100},
			[]int64{50},
			[]string{"not-a-number"},
		)
		_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.ErrorContains(t, err, "cannot parse fee")
	})
}

func TestVerifyFeeProofConcurrentErrors(t *testing.T) {
	t.Run("missing pay event under concurrency", func(t *testing.T) {
		// Seed only nonce 100, request 100-101.
		f := setupFeeProofFixture(t, "fp_conc_nopay",
			[]uint64{100},
			[]int64{50},
			[]string{"10"},
		)
		const concurrency = 50
		type callResult struct{ err error }
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := range concurrency {
			go func(idx int) {
				defer wg.Done()
				_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
					OpType:         f.opType,
					SenderAddress:  "rSender",
					FromNonce:      100,
					ToNonce:        101, // nonce 101 missing
					UntilTimestamp: 1800000000,
				})
				results[idx] = callResult{err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			require.ErrorIs(t, r.err, ErrMissingPayEvent, "caller %d", i)
		}
	})

	t.Run("malformed tx fee under concurrency", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_conc_badfee",
			[]uint64{100},
			[]int64{50},
			[]string{"not-a-number"},
		)
		const concurrency = 50
		type callResult struct{ err error }
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := range concurrency {
			go func(idx int) {
				defer wg.Done()
				_, err := f.verifier.Verify(context.Background(), connector.IPMWFeeProofRequestBody{
					OpType:         f.opType,
					SenderAddress:  "rSender",
					FromNonce:      100,
					ToNonce:        100,
					UntilTimestamp: 1800000000,
				})
				results[idx] = callResult{err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			require.ErrorContains(t, r.err, "cannot parse fee", "caller %d", i)
		}
	})
}
