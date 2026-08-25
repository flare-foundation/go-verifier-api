package btcverifier

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/batchtx"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// These tests exercise the ACTUAL production message path — PaymentBatchedMessages
// backed by a real SQLite event store — end to end through Verify, rather than the
// deferred TeeInstructionMessages path the other Verify tests use.

var channelAddr = common.HexToAddress("0x00000000000000000000000000000000000000C1")

func channelAddrStored() string {
	return strings.ToLower(strings.TrimPrefix(channelAddr.Hex(), "0x"))
}

func memDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Log{}))
	return db
}

// insertPaymentBatched stores one PaymentBatched event row the way the indexer
// would: topic0 = event signature, topic1 = instruction id, topic2 = paymentId,
// data = hex of the ABI-packed message envelope.
func insertPaymentBatched(t *testing.T, db *gorm.DB, instructionID common.Hash, msg payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, logIndex uint64) {
	t.Helper()
	msgBytes, err := structs.Encode(utxoArg(t), msg)
	require.NoError(t, err)
	data, err := cspABI(t).Events[PaymentBatchedEvent].Inputs.NonIndexed().Pack(msgBytes)
	require.NoError(t, err)
	require.NoError(t, db.Create(&database.Log{
		Topic0:          strings.TrimPrefix(cspABI(t).Events[PaymentBatchedEvent].ID.Hex(), "0x"),
		Topic1:          strings.TrimPrefix(instructionID.Hex(), "0x"),
		Topic2:          strings.TrimPrefix(common.BigToHash(new(big.Int).SetUint64(msg.PaymentId)).Hex(), "0x"),
		Data:            hex.EncodeToString(data),
		Address:         channelAddrStored(),
		TransactionHash: strings.Repeat("0", 63) + string(rune('a'+logIndex)),
		LogIndex:        logIndex,
		BlockNumber:     10 + logIndex,
		Timestamp:       1700000000,
	}).Error)
}

func productionVerifier(t *testing.T, db *gorm.DB, src batchSource, res Resolver) *BtcVerifier {
	t.Helper()
	repo := paymentdb.NewDBRepo(nil, db, channelAddr)
	cfg := &config.PMWPaymentStatusConfig{}
	cfg.SourceIDPair = config.SourceIDEncodedPair{SourceID: config.SourceTestBTC, SourceIDEncoded: testSourceID}
	return &BtcVerifier{
		Repo:     repo,
		Messages: PaymentBatchedMessages{Repo: repo, ABI: cspABI(t)},
		Source:   src,
		Resolver: res,
		Config:   cfg,
		Params:   testParams,
	}
}

// testBatchID is the batch payment id shared by the integration fixtures; the
// instruction id is derived from it.
const testBatchID uint64 = 900000

func instructionIDFor(t *testing.T) common.Hash {
	t.Helper()
	id, err := teeinstruction.GenerateInstructionID(testOpType, testSourceID, testAccount, testBatchID)
	require.NoError(t, err)
	return id
}

func TestIntegration_ProductionPath_Success(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce, batchID = 150000, uint64(7), uint64(900000)
	db := memDB(t)
	insertPaymentBatched(t, db, instructionIDFor(t), sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1), 0)

	v := productionVerifier(t, db,
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, nonce, out(recipScript, amount))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(0), resp.TransactionStatus)
	require.Equal(t, recipAddr, resp.RecipientAddress)
	require.Equal(t, big.NewInt(amount), resp.ReceivedAmount)
}

func TestIntegration_ProductionPath_NotFound(t *testing.T) {
	recipAddr, _ := recipient(t)
	const nonce, batchID = uint64(7), uint64(900000)
	db := memDB(t)
	insertPaymentBatched(t, db, instructionIDFor(t), sampleMsg(recipAddr, 150000, batchID, batchID, nonce, 1), 0)

	v := productionVerifier(t, db,
		stubIndexer{anchorAddr: testAnchor, batch: nil}, // settling batch not located
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(2), resp.TransactionStatus)
}

func TestIntegration_ProductionPath_SubDust(t *testing.T) {
	recipAddr, _ := recipient(t)
	const nonce, batchID = uint64(7), uint64(900000)
	db := memDB(t)
	// 100 sat is below the P2WPKH dust threshold, so a zero delivery is undeliverable.
	insertPaymentBatched(t, db, instructionIDFor(t), sampleMsg(recipAddr, 100, batchID, batchID, nonce, 1), 0)

	v := productionVerifier(t, db,
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, nonce, out([]byte{0x52}, 250))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(1), resp.TransactionStatus)
	require.Equal(t, "sub_dust_amount", resp.RevertReason)
}

func TestIntegration_ProductionPath_ConflictingReissue(t *testing.T) {
	recipAddr, _ := recipient(t)
	const nonce, batchID = uint64(7), uint64(900000)
	db := memDB(t)
	// Two emissions for the same paymentId that disagree on amount — a reissue
	// that contradicts itself, which must fail closed rather than pick one.
	id := instructionIDFor(t)
	insertPaymentBatched(t, db, id, sampleMsg(recipAddr, 150000, batchID, batchID, nonce, 1), 0)
	insertPaymentBatched(t, db, id, sampleMsg(recipAddr, 999999, batchID, batchID, nonce, 1), 1)

	v := productionVerifier(t, db,
		stubIndexer{anchorAddr: testAnchor},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}
