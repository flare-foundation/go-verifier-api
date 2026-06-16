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
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
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
		resp, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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
		resp, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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

	t.Run("pay event inconsistent with topic returns DB error", func(t *testing.T) {
		teeABI := testTeeABI(t)
		xrpDB := testSharedDB(t, "fp_inconsist_pay_xrp", &paymentdb.DBTransaction{})
		cChainDB := testSharedDB(t, "fp_inconsist_pay_cchain", &database.Log{})
		sourceID := common.HexToHash("0x1")
		opType := common.HexToHash("0xAA")
		nonce := uint64(100)

		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, "rSender", nonce)
		require.NoError(t, err)
		eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
		require.NoError(t, err)
		// Topic2 commits to sourceID 0x1, but the stored message carries 0x2.
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SourceId: common.HexToHash("0x2"), SenderAddress: "rSender",
			Amount: big.NewInt(1000), MaxFee: big.NewInt(50), TokenId: []byte{}, FeeSchedule: []byte{},
			Nonce: nonce, SubNonce: nonce,
		}
		require.NoError(t, cChainDB.Create(&database.Log{
			Topic0: trimHex(eventHash), Topic1: trimHex(common.HexToHash("").Hex()), Topic2: trimHex(payID.Hex()),
			Data:    hex.EncodeToString(testEncodeEvent(t, teeABI, op.Pay, opType, msg)),
			Address: testContractAddressStored, TransactionHash: fmt.Sprintf("%064x", nonce), LogIndex: nonce,
			Timestamp: 1700000000, BlockNumber: 100,
		}).Error)
		v := &XRPVerifier{
			Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: &config.PMWFeeProofConfig{ParsedTeeInstructionsABI: teeABI, EncodedAndABI: config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}}},
		}
		_, err = v.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType: opType, SenderAddress: "rSender", FromNonce: nonce, ToNonce: nonce, UntilTimestamp: 1800000000,
		})
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "event SourceId")
	})

	t.Run("reissue event inconsistent with topic returns DB error", func(t *testing.T) {
		// Consistent pay event, then a reissue whose message disagrees with its topic.
		f := setupFeeProofFixture(t, "fp_inconsist_reissue", []uint64{100}, []int64{50}, []string{"12"})
		eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(f.teeABI)
		require.NoError(t, err)
		reissueID, err := instruction.GenerateReissueInstructionID(f.opType, f.sourceID, "rSender", 100, 0)
		require.NoError(t, err)
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SourceId: common.HexToHash("0x2"), SenderAddress: "rSender", // SourceId mismatch
			Amount: big.NewInt(1000), MaxFee: big.NewInt(60), TokenId: []byte{}, FeeSchedule: []byte{},
			Nonce: 100, SubNonce: 100,
		}
		require.NoError(t, f.cChainDB.Create(&database.Log{
			Topic0: trimHex(eventHash), Topic1: trimHex(common.HexToHash("").Hex()), Topic2: trimHex(reissueID.Hex()),
			Data:    hex.EncodeToString(testEncodeEvent(t, f.teeABI, op.Reissue, f.opType, msg)),
			Address: testContractAddressStored, TransactionHash: "reissuehash", LogIndex: 100_000_000,
			Timestamp: 1700000000, BlockNumber: 100,
		}).Error)
		_, err = f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType: f.opType, SenderAddress: "rSender", FromNonce: 100, ToNonce: 100, UntilTimestamp: 1800000000,
		})
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "event SourceId")
	})

	t.Run("missing pay event returns error", func(t *testing.T) {
		// Seed nonces 100 and 101, but request 100-102.
		f := setupFeeProofFixture(t, "fp_missing_pay",
			[]uint64{100, 101},
			[]int64{50, 60},
			[]string{"10", "15"},
		)
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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

		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SourceId:      sourceID,
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         nonce,
			SubNonce:      nonce,
		}
		eventData := testEncodeEvent(t, teeABI, op.Pay, opType, msg)

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

		_, err = v.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType:        f.opType,
			SenderAddress: "rSender",
			FromNonce:     10,
			ToNonce:       5,
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
	})

	t.Run("range exceeds max returns error", func(t *testing.T) {
		f := setupFeeProofFixture(t, "fp_bigrange", []uint64{100}, []int64{50}, []string{"10"})
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.ErrorContains(t, err, "cannot parse fee")
	})

	t.Run("negative tx fee rejected", func(t *testing.T) {
		// Corrupted indexer row with a negative Fee value must not parse.
		f := setupFeeProofFixture(t, "fp_negfee",
			[]uint64{100},
			[]int64{50},
			[]string{"-1"},
		)
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.ErrorContains(t, err, "cannot parse fee")
		require.ErrorContains(t, err, "expected non-negative big.Int")
	})

	t.Run("reissue scan at cap succeeds", func(t *testing.T) {
		// Seed pay + exactly MaxReissuesPerNonce reissue events. The next
		// reissueNumber (== MaxReissuesPerNonce) does NOT exist, so the loop
		// terminates cleanly at the cap.
		f := setupFeeProofFixture(t, "fp_reissue_at_cap",
			[]uint64{100},
			[]int64{50}, // pay maxFee = 50
			[]string{"10"},
		)
		for i := range MaxReissuesPerNonce {
			f.seedReissue(t, 100, i, 60, 1700000000) // reissue maxFee = 60 → residual 10 each
		}
		resp, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.NoError(t, err)
		// pay 50 + 32 reissues × residual 10 = 50 + 320 = 370
		require.Equal(t, big.NewInt(50+int64(MaxReissuesPerNonce)*10), resp.EstimatedFee)
	})

	t.Run("reissue scan over cap rejected", func(t *testing.T) {
		// Seed pay + (MaxReissuesPerNonce + 1) sequential reissues. The next
		// reissueNumber (== MaxReissuesPerNonce) exists in the indexer, so the
		// loop must reject with ErrReissueLimitExceeded rather than silently
		// truncate.
		f := setupFeeProofFixture(t, "fp_reissue_over_cap",
			[]uint64{100},
			[]int64{50},
			[]string{"10"},
		)
		for i := uint64(0); i <= MaxReissuesPerNonce; i++ {
			f.seedReissue(t, 100, i, 60, 1700000000)
		}
		_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
			OpType:         f.opType,
			SenderAddress:  "rSender",
			FromNonce:      100,
			ToNonce:        100,
			UntilTimestamp: 1800000000,
		})
		require.ErrorIs(t, err, ErrReissueLimitExceeded)
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
				_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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
				_, err := f.verifier.Verify(context.Background(), fdc2.IPMWFeeProofRequestBody{
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
