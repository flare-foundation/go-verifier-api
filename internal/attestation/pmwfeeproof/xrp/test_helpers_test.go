package xrpverifier

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testContractAddress is the canonical TeeInstructionsSent emitter used in tests.
var testContractAddress = common.HexToAddress("0x00000000000000000000000000000000000000C1")

// testContractAddressStored matches the indexer's lowercase-no-prefix storage format.
const testContractAddressStored = "00000000000000000000000000000000000000c1"

func testSharedDB(tb testing.TB, name string, models ...any) *gorm.DB {
	tb.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		tb.Fatal(err)
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			tb.Fatal(err)
		}
	}
	return db
}

func testTeeABI(tb testing.TB) abi.ABI {
	tb.Helper()
	parsed, err := abi.JSON(strings.NewReader(instructions.InstructionsMetaData.ABI))
	if err != nil {
		tb.Fatal(err)
	}
	return parsed
}

func testEncodeEvent(tb testing.TB, teeABI abi.ABI, command op.Command, opType common.Hash, msg payments.ITeePaymentsPaymentInstructionMessage) []byte {
	tb.Helper()
	msgArg := payments.MessageArguments[command]
	msgBytes, err := structs.Encode(msgArg, msg)
	if err != nil {
		tb.Fatalf("cannot encode message: %v", err)
	}
	opCommand, err := convert.StringToCommonHash(string(command))
	if err != nil {
		tb.Fatalf("cannot hash command: %v", err)
	}
	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]instructions.IMachineManagerTeeMachine{},
		[32]byte(opType), [32]byte(opCommand),
		msgBytes,
		[]common.Address{}, uint64(0), common.Address{}, big.NewInt(0),
	)
	if err != nil {
		tb.Fatalf("cannot pack event data: %v", err)
	}
	return data
}

func trimHex(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

type feeProofFixture struct {
	verifier *XRPVerifier
	opType   common.Hash
	sourceID common.Hash
	cChainDB *gorm.DB
	teeABI   abi.ABI
}

// xrpSequenceFor maps a small paymentId to the (distinct, large) XRP Sequence
// that the corresponding instruction message carries in its Nonce field and
// that the XRP transaction is stored under. Keeping paymentId and Sequence
// distinct ensures the verifier's paymentId -> event -> Nonce -> XRP-tx lookup
// is genuinely exercised; a bug that confuses the two is caught.
const xrpSequenceBase = 11263144

func xrpSequenceFor(paymentId uint64) uint64 {
	return xrpSequenceBase + paymentId
}

// setupFeeProofFixture seeds, for each paymentId, a PAY instruction event keyed
// by the (small) paymentId and an XRP transaction keyed by the (distinct, large)
// Sequence = xrpSequenceFor(paymentId). The message Nonce equals that Sequence.
func setupFeeProofFixture(tb testing.TB, dbName string, paymentIds []uint64, maxFees []int64, txFees []string) feeProofFixture {
	tb.Helper()
	teeABI := testTeeABI(tb)
	xrpDB := testSharedDB(tb, dbName+"_xrp", &paymentdb.DBTransaction{})
	cChainDB := testSharedDB(tb, dbName+"_cchain", &database.Log{})

	sourceID := common.HexToHash("0x1")
	opType := common.HexToHash("0xAA")
	senderAddress := "rSender"

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
	if err != nil {
		tb.Fatal(err)
	}

	for i, paymentId := range paymentIds {
		sequence := xrpSequenceFor(paymentId)
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, senderAddress, paymentId)
		if err != nil {
			tb.Fatal(err)
		}

		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SourceId:      sourceID,
			SenderAddress: senderAddress,
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(maxFees[i]),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         sequence,
			PaymentId:     paymentId,
		}
		eventData := testEncodeEvent(tb, teeABI, op.Pay, opType, msg)

		if err := cChainDB.Create(&database.Log{
			Topic0:          trimHex(eventHash),
			Topic1:          trimHex(common.HexToHash("").Hex()),
			Topic2:          trimHex(payID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", paymentId),
			LogIndex:        paymentId,
			Timestamp:       1700000000,
			BlockNumber:     100,
		}).Error; err != nil {
			tb.Fatal(err)
		}

		if err := xrpDB.Create(&paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("txhash%d", sequence),
			BlockNumber:   100,
			Timestamp:     1700000000,
			Response:      fmt.Sprintf(`{"Fee":"%s","Account":"%s","Sequence":%d,"hash":"txhash%d"}`, txFees[i], senderAddress, sequence, sequence),
			SourceAddress: senderAddress,
			Sequence:      sequence,
		}).Error; err != nil {
			tb.Fatal(err)
		}
	}

	cfg := &config.PMWFeeProofConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}

	return feeProofFixture{
		verifier: &XRPVerifier{
			Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: cfg,
		},
		opType:   opType,
		sourceID: sourceID,
		cChainDB: cChainDB,
		teeABI:   teeABI,
	}
}

// seedReissue inserts a single reissue event for (paymentId, reissueNumber)
// into the fixture's C-chain DB. The message Nonce carries the distinct XRP
// Sequence (xrpSequenceFor(paymentId)). Used by reissue-cap tests to construct
// sequential scans of arbitrary depth.
func (f feeProofFixture) seedReissue(tb testing.TB, paymentId, reissueNumber uint64, maxFee int64, blockTimestamp uint64) {
	tb.Helper()
	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(f.teeABI)
	if err != nil {
		tb.Fatal(err)
	}
	reissueID, err := instruction.GenerateReissueInstructionID(f.opType, f.sourceID, "rSender", paymentId, reissueNumber)
	if err != nil {
		tb.Fatal(err)
	}
	msg := payments.ITeePaymentsPaymentInstructionMessage{
		SourceId:      f.sourceID,
		SenderAddress: "rSender",
		Amount:        big.NewInt(1000),
		MaxFee:        big.NewInt(maxFee),
		TokenId:       []byte{},
		FeeSchedule:   []byte{},
		Nonce:         xrpSequenceFor(paymentId),
		PaymentId:     paymentId,
	}
	eventData := testEncodeEvent(tb, f.teeABI, op.Reissue, f.opType, msg)
	logIdx := paymentId*1_000_000 + reissueNumber
	if err := f.cChainDB.Create(&database.Log{
		Topic0:          trimHex(eventHash),
		Topic1:          trimHex(common.HexToHash("").Hex()),
		Topic2:          trimHex(reissueID.Hex()),
		Data:            hex.EncodeToString(eventData),
		Address:         testContractAddressStored,
		TransactionHash: fmt.Sprintf("re%d-%d", paymentId, reissueNumber),
		LogIndex:        logIdx,
		Timestamp:       blockTimestamp,
		BlockNumber:     100,
	}).Error; err != nil {
		tb.Fatal(err)
	}
}
