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
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSharedDB(t *testing.T, name string, models ...any) *gorm.DB {
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

func testTeeABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func testEncodeEvent(t *testing.T, teeABI abi.ABI, command op.Command, msg payment.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()
	msgArg := payment.MessageArguments[command]
	msgBytes, err := structs.Encode(msgArg, msg)
	if err != nil {
		t.Fatalf("cannot encode message: %v", err)
	}
	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]teeextensionregistry.ITeeMachineRegistryTeeMachine{},
		[32]byte{}, [32]byte{},
		msgBytes,
		[]common.Address{}, uint64(0), common.Address{}, big.NewInt(0),
	)
	if err != nil {
		t.Fatalf("cannot pack event data: %v", err)
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

func setupFeeProofFixture(t *testing.T, dbName string, nonces []uint64, maxFees []int64, txFees []string) feeProofFixture {
	t.Helper()
	teeABI := testTeeABI(t)
	xrpDB := testSharedDB(t, dbName+"_xrp", &paymentdb.DBTransaction{})
	cChainDB := testSharedDB(t, dbName+"_cchain", &database.Log{})

	sourceID := common.HexToHash("0x1")
	opType := common.HexToHash("0xAA")
	senderAddress := "rSender"

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
	if err != nil {
		t.Fatal(err)
	}

	for i, nonce := range nonces {
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, senderAddress, nonce)
		if err != nil {
			t.Fatal(err)
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
		eventData := testEncodeEvent(t, teeABI, op.Pay, msg)

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
			t.Fatal(err)
		}

		if err := xrpDB.Create(&paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("txhash%d", nonce),
			BlockNumber:   100,
			Timestamp:     1700000000,
			Response:      fmt.Sprintf(`{"Fee":"%s"}`, txFees[i]),
			SourceAddress: senderAddress,
			Sequence:      nonce,
		}).Error; err != nil {
			t.Fatal(err)
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
