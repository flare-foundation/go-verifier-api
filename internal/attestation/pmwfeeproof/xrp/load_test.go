//go:build load

package xrpverifier

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSharedMemDB(t *testing.T, name string, models ...any) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func loadABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(instructions.InstructionsMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func encodePaymentEventData(t *testing.T, teeABI abi.ABI, command op.Command, opType common.Hash, msg payments.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()
	msgArg := payments.MessageArguments[command]
	msgBytes, err := structs.Encode(msgArg, msg)
	if err != nil {
		t.Fatalf("cannot encode message: %v", err)
	}
	opCommand, err := convert.StringToCommonHash(string(command))
	if err != nil {
		t.Fatalf("cannot hash command: %v", err)
	}
	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]instructions.IMachineManagerTeeMachine{},
		[32]byte(opType),
		[32]byte(opCommand),
		msgBytes,
		[]common.Address{},
		uint64(0),
		common.Address{},
		big.NewInt(0),
	)
	if err != nil {
		t.Fatalf("cannot pack event data: %v", err)
	}
	return data
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

// TestLoadFeeProofConcurrentVerify simulates concurrent fee proof verification
// through the full verifier flow: batch event fetch, tx fee sum, and the no-reissue path.
func TestLoadFeeProofConcurrentVerify(t *testing.T) {
	teeABI := loadABI(t)
	xrpDB := newSharedMemDB(t, "fp_verify_xrp", &paymentdb.DBTransaction{})
	cChainDB := newSharedMemDB(t, "fp_verify_cchain", &database.Log{})

	sourceID := common.HexToHash("0x1")
	cfg := &config.PMWFeeProofConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}

	v := &XRPVerifier{
		Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
		Config: cfg,
	}

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
	if err != nil {
		t.Fatal(err)
	}

	opType := common.HexToHash("0xAA")
	senderAddress := "rSender"
	fromNonce := uint64(100)
	toNonce := uint64(104) // 5 nonces

	// Seed pay events and transactions for each nonce.
	for nonce := fromNonce; nonce <= toNonce; nonce++ {
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, senderAddress, nonce)
		if err != nil {
			t.Fatal(err)
		}

		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SourceId:         sourceID,
			SenderAddress:    senderAddress,
			RecipientAddress: "rRecipient",
			Amount:           big.NewInt(1000),
			MaxFee:           big.NewInt(int64(50 + nonce - fromNonce)),
			TokenId:          []byte{},
			FeeSchedule:      []byte{},
			Nonce:            nonce,
			SubNonce:         nonce,
		}
		eventData := encodePaymentEventData(t, teeABI, op.Pay, opType, msg)

		log := database.Log{
			Topic0:          removeHexPrefix(eventHash),
			Topic1:          removeHexPrefix(common.HexToHash("").Hex()),
			Topic2:          removeHexPrefix(payID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
			Timestamp:       1700000000,
			BlockNumber:     100,
		}
		if err := cChainDB.Create(&log).Error; err != nil {
			t.Fatal(err)
		}

		tx := paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("txhash%d", nonce),
			BlockNumber:   100,
			Timestamp:     1700000000,
			Response:      fmt.Sprintf(`{"Fee":"%d","Account":"%s","Sequence":%d,"hash":"txhash%d"}`, 10+nonce-fromNonce, senderAddress, nonce, nonce),
			SourceAddress: senderAddress,
			Sequence:      nonce,
		}
		if err := xrpDB.Create(&tx).Error; err != nil {
			t.Fatal(err)
		}
	}

	req := fdc2.IPMWFeeProofRequestBody{
		OpType:         opType,
		SenderAddress:  senderAddress,
		FromNonce:      fromNonce,
		ToNonce:        toNonce,
		UntilTimestamp: 1800000000,
	}

	const (
		concurrency = 100
		rounds      = 10
	)

	type callResult struct {
		resp fdc2.IPMWFeeProofResponseBody
		err  error
	}

	var allLatencies []time.Duration
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				resp, err := v.Verify(context.Background(), req)
				elapsed := time.Since(start)
				mu.Lock()
				allLatencies = append(allLatencies, elapsed)
				mu.Unlock()
				results[idx] = callResult{resp: resp, err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			if r.err != nil {
				t.Fatalf("round %d, caller %d: %v", round, i, r.err)
			}
			if r.resp.ActualFee == nil || r.resp.EstimatedFee == nil {
				t.Fatalf("round %d, caller %d: nil fee in response", round, i)
			}
		}

		// All callers in the same round should get identical results.
		for i := 1; i < concurrency; i++ {
			if results[i].resp.ActualFee.Cmp(results[0].resp.ActualFee) != 0 {
				t.Fatalf("round %d: inconsistent actualFee across callers", round)
			}
			if results[i].resp.EstimatedFee.Cmp(results[0].resp.EstimatedFee) != 0 {
				t.Fatalf("round %d: inconsistent estimatedFee across callers", round)
			}
		}
	}

	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })
	n := len(allLatencies)
	t.Logf("FeeProof verifier: n=%d, p50=%v, p95=%v, p99=%v",
		n, allLatencies[n*50/100], allLatencies[n*95/100], allLatencies[n*99/100])
}
