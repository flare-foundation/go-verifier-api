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
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
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
	parsed, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func encodePaymentEventData(t *testing.T, teeABI abi.ABI, msg payment.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()

	msgArg := payment.MessageArguments[op.Pay]
	msgBytes, err := structs.Encode(msgArg, msg)
	if err != nil {
		t.Fatalf("cannot encode payment message: %v", err)
	}

	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]teeextensionregistry.ITeeMachineRegistryTeeMachine{}, // TeeMachines
		[32]byte{},         // OpType
		[32]byte{},         // OpCommand
		msgBytes,           // Message
		[]common.Address{}, // Cosigners
		uint64(0),          // CosignersThreshold
		common.Address{},   // ClaimBackAddress
		big.NewInt(0),      // Fee
	)
	if err != nil {
		t.Fatalf("cannot pack event data: %v", err)
	}
	return data
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

func seedTestData(
	t *testing.T,
	teeABI abi.ABI,
	xrpDB, cChainDB *gorm.DB,
	eventHash string,
	instructionID common.Hash,
	senderAddress string,
	recipientAddress string,
	nonce uint64,
	amount *big.Int,
	maxFee *big.Int,
	txFee string,
) {
	t.Helper()

	msg := payment.ITeePaymentsPaymentInstructionMessage{
		SenderAddress:    senderAddress,
		RecipientAddress: recipientAddress,
		Amount:           amount,
		MaxFee:           maxFee,
		TokenId:          []byte{},
		FeeSchedule:      []byte{},
		Nonce:            nonce,
		SubNonce:         nonce,
	}
	eventData := encodePaymentEventData(t, teeABI, msg)

	log := database.Log{
		Topic0:          removeHexPrefix(eventHash),
		Topic1:          removeHexPrefix(common.HexToHash("").Hex()),
		Topic2:          removeHexPrefix(instructionID.Hex()),
		Data:            hex.EncodeToString(eventData),
		Address:         testContractAddressStored,
		TransactionHash: fmt.Sprintf("%064x", nonce),
		LogIndex:        nonce,
		Timestamp:       1700000000,
		BlockNumber:     100,
	}
	if err := cChainDB.Create(&log).Error; err != nil {
		t.Fatalf("cannot seed log: %v", err)
	}

	txResponse := fmt.Sprintf(
		`{"Account":"%s","Amount":"%s","Destination":"%s","Fee":"%s","Sequence":%d,"TransactionType":"Payment","metaData":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"%s","Balance":"1000000"},"LedgerEntryType":"AccountRoot","PreviousFields":{"Balance":"900000"}}}],"TransactionResult":"tesSUCCESS","delivered_amount":"%s"}}`,
		senderAddress, amount.String(), recipientAddress, txFee, nonce, recipientAddress, amount.String())

	tx := paymentdb.DBTransaction{
		Hash:          fmt.Sprintf("%064x", nonce),
		BlockNumber:   100,
		Timestamp:     1700000000,
		Response:      txResponse,
		SourceAddress: senderAddress,
		Sequence:      nonce,
	}
	if err := xrpDB.Create(&tx).Error; err != nil {
		t.Fatalf("cannot seed transaction: %v", err)
	}
}

// TestLoadPaymentStatusConcurrentVerify simulates concurrent verification requests
// through the full verifier flow including ABI decode, DB reads, and response building.
func TestLoadPaymentStatusConcurrentVerify(t *testing.T) {
	teeABI := loadABI(t)
	xrpDB := newSharedMemDB(t, "ps_verify_xrp", &paymentdb.DBTransaction{})
	cChainDB := newSharedMemDB(t, "ps_verify_cchain", &database.Log{})

	sourceID := common.HexToHash("0x1")
	cfg := &config.PMWPaymentStatusConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}

	v := &XRPVerifier{
		Repo:   paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
		Config: cfg,
	}

	eventHash, err := instruction.TeeInstructionsSentEventSignature(teeABI)
	if err != nil {
		t.Fatal(err)
	}

	opType := common.HexToHash("0xAA")
	senderAddress := "rSender"
	nonce := uint64(42)

	instructionID, err := instruction.GenerateInstructionID(opType, sourceID, senderAddress, nonce)
	if err != nil {
		t.Fatal(err)
	}

	seedTestData(t, teeABI, xrpDB, cChainDB, eventHash, instructionID,
		senderAddress, "rRecipient", nonce, big.NewInt(1000), big.NewInt(50), "12")

	req := connector.IPMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: senderAddress,
		Nonce:         nonce,
		SubNonce:      nonce,
	}

	const (
		concurrency = 100
		rounds      = 10
	)

	type callResult struct {
		resp connector.IPMWPaymentStatusResponseBody
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
			if r.resp.TransactionStatus != 0 {
				t.Fatalf("round %d, caller %d: expected status 0, got %d", round, i, r.resp.TransactionStatus)
			}
		}

		// All callers in the same round should get identical results.
		for i := 1; i < concurrency; i++ {
			if results[i].resp.TransactionStatus != results[0].resp.TransactionStatus {
				t.Fatalf("round %d: inconsistent TransactionStatus across callers", round)
			}
			if results[i].resp.RecipientAddress != results[0].resp.RecipientAddress {
				t.Fatalf("round %d: inconsistent RecipientAddress across callers", round)
			}
			if results[i].resp.TransactionFee.Cmp(results[0].resp.TransactionFee) != 0 {
				t.Fatalf("round %d: inconsistent TransactionFee across callers", round)
			}
		}
	}

	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })
	n := len(allLatencies)
	t.Logf("PaymentStatus verifier: n=%d, p50=%v, p95=%v, p99=%v",
		n, allLatencies[n*50/100], allLatencies[n*95/100], allLatencies[n*99/100])
}
