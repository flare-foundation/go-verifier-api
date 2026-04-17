package xrpverifier

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-flare-common/pkg/xrpl/transactions"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testContractAddress is the canonical TeeInstructionsSent emitter address used in tests.
var testContractAddress = common.HexToAddress("0x00000000000000000000000000000000000000C1")

// testContractAddressStored matches the indexer's lowercase-no-prefix storage format.
const testContractAddressStored = "00000000000000000000000000000000000000c1"

func newTestDB(t *testing.T, name string, models ...any) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, m := range models {
		require.NoError(t, db.AutoMigrate(m))
	}
	return db
}

func testABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	require.NoError(t, err)
	return parsed
}

func encodeEventData(t *testing.T, teeABI abi.ABI, msg payment.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()
	msgArg := payment.MessageArguments[op.Pay]
	msgBytes, err := structs.Encode(msgArg, msg)
	require.NoError(t, err)
	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]teeextensionregistry.ITeeMachineRegistryTeeMachine{},
		[32]byte{}, [32]byte{},
		msgBytes,
		[]common.Address{}, uint64(0), common.Address{}, big.NewInt(0),
	)
	require.NoError(t, err)
	return data
}

func stripHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

type testFixture struct {
	verifier *XRPVerifier
	req      connector.IPMWPaymentStatusRequestBody
}

func setupVerifyFixture(t *testing.T, dbName string, txResponse string) testFixture {
	t.Helper()
	teeABI := testABI(t)
	xrpDB := newTestDB(t, dbName+"_xrp", &paymentdb.DBTransaction{})
	cChainDB := newTestDB(t, dbName+"_cchain", &database.Log{})

	sourceID := common.HexToHash("0x1")
	cfg := &config.PMWPaymentStatusConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}

	opType := common.HexToHash("0xAA")
	senderAddress := "rSender"
	nonce := uint64(42)

	instructionID, err := instruction.GenerateInstructionID(opType, sourceID, senderAddress, nonce)
	require.NoError(t, err)

	eventHash, err := instruction.TeeInstructionsSentEventSignature(teeABI)
	require.NoError(t, err)

	msg := payment.ITeePaymentsPaymentInstructionMessage{
		SenderAddress:    senderAddress,
		RecipientAddress: "rRecipient",
		Amount:           big.NewInt(1000),
		MaxFee:           big.NewInt(50),
		TokenId:          []byte{},
		FeeSchedule:      []byte{},
		Nonce:            nonce,
		SubNonce:         nonce,
	}
	eventData := encodeEventData(t, teeABI, msg)

	require.NoError(t, cChainDB.Create(&database.Log{
		Topic0:          stripHexPrefix(eventHash),
		Topic1:          stripHexPrefix(common.HexToHash("").Hex()),
		Topic2:          stripHexPrefix(instructionID.Hex()),
		Data:            hex.EncodeToString(eventData),
		Address:         testContractAddressStored,
		TransactionHash: fmt.Sprintf("%064x", nonce),
		LogIndex:        nonce,
		Timestamp:       1700000000,
		BlockNumber:     100,
	}).Error)

	require.NoError(t, xrpDB.Create(&paymentdb.DBTransaction{
		Hash:          fmt.Sprintf("%064x", nonce),
		BlockNumber:   100,
		Timestamp:     1700000000,
		Response:      txResponse,
		SourceAddress: senderAddress,
		Sequence:      nonce,
	}).Error)

	return testFixture{
		verifier: &XRPVerifier{
			Repo:   paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: cfg,
		},
		req: connector.IPMWPaymentStatusRequestBody{
			OpType:        opType,
			SenderAddress: senderAddress,
			Nonce:         nonce,
			SubNonce:      nonce,
		},
	}
}

func TestVerifyConcurrentErrors(t *testing.T) {
	t.Run("missing log under concurrency", func(t *testing.T) {
		teeABI := testABI(t)
		xrpDB := newTestDB(t, "conc_nolog_xrp", &paymentdb.DBTransaction{})
		cChainDB := newTestDB(t, "conc_nolog_cchain", &database.Log{})

		v := &XRPVerifier{
			Repo: paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: &config.PMWPaymentStatusConfig{
				ParsedTeeInstructionsABI: teeABI,
				EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: common.HexToHash("0x1")}},
			},
		}
		req := connector.IPMWPaymentStatusRequestBody{
			OpType: common.HexToHash("0xAA"), SenderAddress: "rSender", Nonce: 999,
		}

		const concurrency = 50
		type callResult struct{ err error }
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := range concurrency {
			go func(idx int) {
				defer wg.Done()
				_, err := v.Verify(context.Background(), req)
				results[idx] = callResult{err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			require.ErrorContains(t, r.err, "record not found", "caller %d", i)
		}
	})

	t.Run("malformed JSON under concurrency", func(t *testing.T) {
		f := setupVerifyFixture(t, "conc_badjson", "not-json-at-all")
		const concurrency = 50
		type callResult struct{ err error }
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := range concurrency {
			go func(idx int) {
				defer wg.Done()
				_, err := f.verifier.Verify(context.Background(), f.req)
				results[idx] = callResult{err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			require.ErrorContains(t, r.err, "cannot unmarshal XRP transaction response", "caller %d", i)
		}
	})
}

const successTxResponse = `{"hash":"000000000000000000000000000000000000000000000000000000000000002a","Account":"rSender","Amount":"1000","Destination":"rRecipient","Fee":"12","Sequence":42,"TransactionType":"Payment","metaData":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"rRecipient","Balance":"2000"},"LedgerEntryType":"AccountRoot","PreviousFields":{"Balance":"1000"}}}],"TransactionResult":"tesSUCCESS","delivered_amount":"1000"}}`

const revertedTxResponse = `{"hash":"000000000000000000000000000000000000000000000000000000000000002a","Account":"rSender","Amount":"1000","Destination":"rRecipient","Fee":"12","Sequence":42,"TransactionType":"Payment","metaData":{"AffectedNodes":[],"TransactionResult":"tecNO_DST_INSUF_XRP"}}`

func TestVerify(t *testing.T) {
	t.Run("successful payment", func(t *testing.T) {
		f := setupVerifyFixture(t, "verify_success", successTxResponse)
		resp, err := f.verifier.Verify(context.Background(), f.req)
		require.NoError(t, err)
		require.Equal(t, uint8(0), resp.TransactionStatus) // success
		require.Equal(t, "rRecipient", resp.RecipientAddress)
		require.Equal(t, "", resp.RevertReason)
		require.NotNil(t, resp.TransactionFee)
		require.Equal(t, big.NewInt(12), resp.TransactionFee)
		require.NotNil(t, resp.Amount)
		require.Equal(t, big.NewInt(1000), resp.Amount)
	})

	t.Run("reverted payment", func(t *testing.T) {
		f := setupVerifyFixture(t, "verify_reverted", revertedTxResponse)
		resp, err := f.verifier.Verify(context.Background(), f.req)
		require.NoError(t, err)
		require.Equal(t, uint8(1), resp.TransactionStatus) // reverted
		require.Equal(t, "tecNO_DST_INSUF_XRP", resp.RevertReason)
		require.Equal(t, big.NewInt(12), resp.TransactionFee)
	})

	t.Run("missing instruction log returns error", func(t *testing.T) {
		teeABI := testABI(t)
		xrpDB := newTestDB(t, "verify_nolog_xrp", &paymentdb.DBTransaction{})
		cChainDB := newTestDB(t, "verify_nolog_cchain", &database.Log{})

		v := &XRPVerifier{
			Repo: paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: &config.PMWPaymentStatusConfig{
				ParsedTeeInstructionsABI: teeABI,
				EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: common.HexToHash("0x1")}},
			},
		}
		req := connector.IPMWPaymentStatusRequestBody{
			OpType:        common.HexToHash("0xAA"),
			SenderAddress: "rSender",
			Nonce:         999,
		}
		_, err := v.Verify(context.Background(), req)
		require.ErrorContains(t, err, "record not found")
	})

	t.Run("missing transaction returns error", func(t *testing.T) {
		teeABI := testABI(t)
		xrpDB := newTestDB(t, "verify_notx_xrp", &paymentdb.DBTransaction{})
		cChainDB := newTestDB(t, "verify_notx_cchain", &database.Log{})

		sourceID := common.HexToHash("0x1")
		opType := common.HexToHash("0xAA")
		senderAddress := "rSender"
		nonce := uint64(42)

		instructionID, err := instruction.GenerateInstructionID(opType, sourceID, senderAddress, nonce)
		require.NoError(t, err)
		eventHash, err := instruction.TeeInstructionsSentEventSignature(teeABI)
		require.NoError(t, err)

		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: senderAddress,
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         nonce,
			SubNonce:      nonce,
		}
		eventData := encodeEventData(t, teeABI, msg)

		require.NoError(t, cChainDB.Create(&database.Log{
			Topic0:          stripHexPrefix(eventHash),
			Topic1:          stripHexPrefix(common.HexToHash("").Hex()),
			Topic2:          stripHexPrefix(instructionID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
		}).Error)
		// No transaction seeded.

		v := &XRPVerifier{
			Repo: paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: &config.PMWPaymentStatusConfig{
				ParsedTeeInstructionsABI: teeABI,
				EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
			},
		}
		req := connector.IPMWPaymentStatusRequestBody{
			OpType: opType, SenderAddress: senderAddress, Nonce: nonce, SubNonce: nonce,
		}
		_, err = v.Verify(context.Background(), req)
		require.ErrorContains(t, err, "record not found")
	})

	t.Run("malformed event data returns decode error", func(t *testing.T) {
		teeABI := testABI(t)
		xrpDB := newTestDB(t, "verify_badevent_xrp", &paymentdb.DBTransaction{})
		cChainDB := newTestDB(t, "verify_badevent_cchain", &database.Log{})

		sourceID := common.HexToHash("0x1")
		opType := common.HexToHash("0xAA")
		senderAddress := "rSender"
		nonce := uint64(42)

		instructionID, err := instruction.GenerateInstructionID(opType, sourceID, senderAddress, nonce)
		require.NoError(t, err)
		eventHash, err := instruction.TeeInstructionsSentEventSignature(teeABI)
		require.NoError(t, err)

		// Seed a log with garbage event data.
		require.NoError(t, cChainDB.Create(&database.Log{
			Topic0:          stripHexPrefix(eventHash),
			Topic1:          stripHexPrefix(common.HexToHash("").Hex()),
			Topic2:          stripHexPrefix(instructionID.Hex()),
			Data:            hex.EncodeToString([]byte("not-abi-encoded")),
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
		}).Error)

		v := &XRPVerifier{
			Repo: paymentdb.NewDBRepo(xrpDB, cChainDB, testContractAddress),
			Config: &config.PMWPaymentStatusConfig{
				ParsedTeeInstructionsABI: teeABI,
				EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
			},
		}
		req := connector.IPMWPaymentStatusRequestBody{
			OpType: opType, SenderAddress: senderAddress, Nonce: nonce, SubNonce: nonce,
		}
		_, err = v.Verify(context.Background(), req)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("malformed transaction JSON returns error", func(t *testing.T) {
		f := setupVerifyFixture(t, "verify_badjson", "not-json")
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorContains(t, err, "cannot unmarshal XRP transaction response")
	})

	t.Run("missing transaction result returns error", func(t *testing.T) {
		f := setupVerifyFixture(t, "verify_noresult", `{"Account":"rSender","Fee":"12","metaData":{"AffectedNodes":[],"TransactionResult":""}}`)
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorContains(t, err, "missing transaction result")
	})

	t.Run("JSON hash mismatch returns 503", func(t *testing.T) {
		resp := `{"hash":"deadbeef","Account":"rSender","Sequence":42,"Fee":"12","TransactionType":"Payment","metaData":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"rRecipient","Balance":"2000"},"LedgerEntryType":"AccountRoot","PreviousFields":{"Balance":"1000"}}}],"TransactionResult":"tesSUCCESS"}}`
		f := setupVerifyFixture(t, "verify_hashmismatch", resp)
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON hash")
	})

	t.Run("JSON Account mismatch returns 503", func(t *testing.T) {
		resp := `{"hash":"000000000000000000000000000000000000000000000000000000000000002a","Account":"rDifferent","Sequence":42,"Fee":"12","TransactionType":"Payment","metaData":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"rRecipient","Balance":"2000"},"LedgerEntryType":"AccountRoot","PreviousFields":{"Balance":"1000"}}}],"TransactionResult":"tesSUCCESS"}}`
		f := setupVerifyFixture(t, "verify_acctmismatch", resp)
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Account")
	})

	t.Run("JSON Sequence mismatch returns 503", func(t *testing.T) {
		resp := `{"hash":"000000000000000000000000000000000000000000000000000000000000002a","Account":"rSender","Sequence":99,"Fee":"12","TransactionType":"Payment","metaData":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"rRecipient","Balance":"2000"},"LedgerEntryType":"AccountRoot","PreviousFields":{"Balance":"1000"}}}],"TransactionResult":"tesSUCCESS"}}`
		f := setupVerifyFixture(t, "verify_seqmismatch", resp)
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Sequence")
	})

	t.Run("oversized response returns 503", func(t *testing.T) {
		// Pad beyond maxResponseSize (1 MB) to trip the size cap before JSON unmarshaling.
		padding := strings.Repeat("x", 1<<20+1)
		resp := `{"_pad":"` + padding + `","hash":"000000000000000000000000000000000000000000000000000000000000002a","Account":"rSender","Sequence":42,"Fee":"12","TransactionType":"Payment","metaData":{"AffectedNodes":[],"TransactionResult":"tesSUCCESS"}}`
		f := setupVerifyFixture(t, "verify_oversized", resp)
		_, err := f.verifier.Verify(context.Background(), f.req)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "too large")
	})
}

func TestCheckRowConsistency(t *testing.T) {
	raw := types.RawTransactionData{
		CommonFields: transactions.CommonFields{
			Account:  "rSender",
			Sequence: 42,
		},
		Hash: "abc123",
	}
	dbTx := paymentdb.DBTransaction{
		Hash:          "abc123",
		SourceAddress: "rSender",
		Sequence:      42,
	}

	t.Run("all match", func(t *testing.T) {
		require.NoError(t, checkRowConsistency(raw, dbTx))
	})

	t.Run("hash case-insensitive match", func(t *testing.T) {
		// Indexer stores Hash lowercase; raw XRPL JSON uses uppercase.
		// EqualFold must treat them as equal.
		r := raw
		r.Hash = "ABC123"
		d := dbTx
		d.Hash = "abc123"
		require.NoError(t, checkRowConsistency(r, d))
	})

	t.Run("empty JSON hash rejected", func(t *testing.T) {
		r := raw
		r.Hash = ""
		err := checkRowConsistency(r, dbTx)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON hash")
	})

	t.Run("hash mismatch", func(t *testing.T) {
		r := raw
		r.Hash = "deadbeef"
		err := checkRowConsistency(r, dbTx)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON hash")
	})

	t.Run("account mismatch", func(t *testing.T) {
		r := raw
		r.Account = "rOther"
		err := checkRowConsistency(r, dbTx)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Account")
	})

	t.Run("sequence mismatch", func(t *testing.T) {
		r := raw
		r.Sequence = 99
		err := checkRowConsistency(r, dbTx)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Sequence")
	})
}
