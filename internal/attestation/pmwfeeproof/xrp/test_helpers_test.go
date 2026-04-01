package xrpverifier

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
	parsed, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		tb.Fatal(err)
	}
	return parsed
}

func testEncodeEvent(tb testing.TB, teeABI abi.ABI, command op.Command, msg payment.ITeePaymentsPaymentInstructionMessage) []byte {
	tb.Helper()
	msgArg := payment.MessageArguments[command]
	msgBytes, err := structs.Encode(msgArg, msg)
	if err != nil {
		tb.Fatalf("cannot encode message: %v", err)
	}
	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]teeextensionregistry.ITeeMachineRegistryTeeMachine{},
		[32]byte{}, [32]byte{},
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
}

func setupFeeProofFixture(tb testing.TB, dbName string, nonces []uint64, maxFees []int64, txFees []string) feeProofFixture {
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

	for i, nonce := range nonces {
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, senderAddress, nonce)
		if err != nil {
			tb.Fatal(err)
		}

		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: senderAddress,
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(maxFees[i]),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         nonce,
			SubNonce:      nonce,
		}
		eventData := testEncodeEvent(tb, teeABI, op.Pay, msg)

		if err := cChainDB.Create(&database.Log{
			Topic0:          trimHex(eventHash),
			Topic1:          trimHex(common.HexToHash("").Hex()),
			Topic2:          trimHex(payID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         "contractAddr",
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
			Timestamp:       1700000000,
			BlockNumber:     100,
		}).Error; err != nil {
			tb.Fatal(err)
		}

		if err := xrpDB.Create(&paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("txhash%d", nonce),
			BlockNumber:   100,
			Timestamp:     1700000000,
			Response:      fmt.Sprintf(`{"Fee":"%s"}`, txFees[i]),
			SourceAddress: senderAddress,
			Sequence:      nonce,
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
			Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB),
			Config: cfg,
		},
		opType:   opType,
		sourceID: sourceID,
	}
}
