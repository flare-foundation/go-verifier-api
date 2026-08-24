package btcverifier

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/stretchr/testify/require"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/batchtx"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// --- stubs -----------------------------------------------------------------

type stubResolver struct {
	id         uint64
	err        error
	anchorTxid string
	anchorVout uint32
	anchorErr  error
}

func (s stubResolver) ResolveBatch(context.Context, common.Hash, string, uint64) (uint64, error) {
	return s.id, s.err
}

func (s stubResolver) ResolveAnchorOutpoint(context.Context, common.Hash, string, uint32) (string, uint32, error) {
	return s.anchorTxid, s.anchorVout, s.anchorErr
}

type stubRepo struct {
	logs []*types.Log
	err  error
}

// Both layouts answer from the same stub: which topic carries the instruction
// id is the repo's concern, and a test that distinguished them here would be
// asserting on the stub rather than on the code under test.
func (s stubRepo) FetchLogsByInstructionTopic1(ctx context.Context, eventHash string, id common.Hash) ([]*types.Log, error) {
	return s.FetchInstructionLogsForID(ctx, eventHash, id)
}

func (s stubRepo) FetchInstructionLogsForID(context.Context, string, common.Hash) ([]*types.Log, error) {
	return s.logs, s.err
}

type stubIndexer struct {
	anchorAddr string
	anchorErr  error
	batch      *batchtx.BatchTx
	batchErr   error

	// gotLocator records what the verifier asked for, so a test can assert the
	// request's txid reaches the source rather than being quietly dropped.
	gotLocator *locator
}

func (s stubIndexer) AnchorAddress(context.Context, string, uint32) (string, error) {
	return s.anchorAddr, s.anchorErr
}

func (s stubIndexer) Batch(_ context.Context, loc locator) (*batchtx.BatchTx, error) {
	if s.gotLocator != nil {
		*s.gotLocator = loc
	}
	return s.batch, s.batchErr
}

// --- fixtures --------------------------------------------------------------

var (
	testParams   = &chaincfg.SigNetParams
	testSourceID = common.HexToHash("0xb7c")
	testOpType   = common.HexToHash("0xaa")
	testAccount  = "bc1qacct"
	testRef      = common.HexToHash("0xfeed")
	testTxid     = strings.Repeat("ab", 32)
	testAnchor   = "bc1qanchor"
)

func teeABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(instructions.InstructionsMetaData.ABI))
	require.NoError(t, err)
	return parsed
}

func utxoArg(t *testing.T) abi.Argument {
	t.Helper()
	pABI, err := payments.TeePaymentsMetaData.GetAbi()
	require.NoError(t, err)
	m, ok := pABI.Methods["utxoPaymentInstructionMessageStruct"]
	require.True(t, ok)
	return m.Inputs[0]
}

func recipient(t *testing.T) (string, []byte) {
	t.Helper()
	addr, err := btcutil.NewAddressWitnessPubKeyHash(make([]byte, 20), testParams)
	require.NoError(t, err)
	script, err := txscript.PayToAddrScript(addr)
	require.NoError(t, err)
	return addr.EncodeAddress(), script
}

func sampleMsg(recipientAddr string, amount int64, paymentID, batchPaymentID, nonce uint64, anchorIndex uint32) payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage {
	return payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage{
		SourceId:         testSourceID,
		AccountAddress:   testAccount,
		AnchorIndex:      anchorIndex,
		RecipientAddress: recipientAddr,
		TokenId:          []byte{},
		Amount:           big.NewInt(amount),
		MaxFee:           big.NewInt(500),
		FeeSchedule:      []byte{},
		PaymentReference: testRef,
		Nonce:            nonce,
		PaymentId:        paymentID,
		BatchPaymentId:   batchPaymentID,
	}
}

func encodeLog(t *testing.T, tABI abi.ABI, opType common.Hash, cmd op.Command, msg payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) *types.Log {
	t.Helper()
	msgBytes, err := structs.Encode(utxoArg(t), msg)
	require.NoError(t, err)
	opCommand, err := convert.StringToCommonHash(string(cmd))
	require.NoError(t, err)
	eventABI := tABI.Events["TeeInstructionsSent"]
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
	require.NoError(t, err)
	return &types.Log{Data: data}
}

func nullData(t *testing.T, data []byte) []byte {
	t.Helper()
	s, err := txscript.NullDataScript(data)
	require.NoError(t, err)
	return s
}

func nonceScript(t *testing.T, nonce uint64) []byte {
	t.Helper()
	payload := append([]byte{0x46, 0x4c, 0x52, 0x00}, binary.BigEndian.AppendUint64(nil, nonce)...)
	return nullData(t, payload)
}

func out(script []byte, sats int64) batchtx.Output {
	return batchtx.Output{PkScript: script, Value: sats}
}

// batchOutputs builds a batch's outputs: fixed prefix (anchor, P2A, nonce),
// one group opener (reference testRef), then the group's value outputs.
func batchOutputs(t *testing.T, nonce uint64, groupOuts ...batchtx.Output) []batchtx.Output {
	t.Helper()
	outs := make([]batchtx.Output, 0, 4+len(groupOuts))
	outs = append(outs,
		out([]byte{0x51}, 1000),         // [0] anchor
		out([]byte{0x51}, 330),          // [1] P2A
		out(nonceScript(t, nonce), 0),   // [2] nonce OP_RETURN
		out(nullData(t, testRef[:]), 0), // [3] group opener
	)
	return append(outs, groupOuts...)
}

func newVerifier(t *testing.T, repo logRepo, idx batchSource, res Resolver) *BtcVerifier {
	t.Helper()
	cfg := &config.PMWPaymentStatusConfig{}
	cfg.SourceIDPair = config.SourceIDEncodedPair{SourceID: config.SourceTestBTC, SourceIDEncoded: testSourceID}
	cfg.ParsedTeeInstructionsABI = teeABI(t)
	return &BtcVerifier{
		Repo: repo,
		// The diamond path: these tests feed TeeInstructionsSent logs, which is
		// what TeePaymentsUtxo emits. The CSP path is covered separately.
		Messages: TeeInstructionMessages{Repo: repo, ABI: cfg.ParsedTeeInstructionsABI},
		Source:   idx,
		Resolver: res,
		Config:   cfg,
		Params:   testParams,
	}
}

func req(paymentID uint64) fdc2.IPMWPaymentStatusRequestBody {
	return fdc2.IPMWPaymentStatusRequestBody{OpType: testOpType, SenderAddress: testAccount, PaymentId: paymentID}
}

// reqWithTxid is req plus the settling-transaction locator.
func reqWithTxid(t *testing.T, paymentID uint64, txid string) fdc2.IPMWPaymentStatusRequestBody {
	t.Helper()
	raw, err := hex.DecodeString(txid)
	require.NoError(t, err)
	r := req(paymentID)
	copy(r.TransactionId[:], raw)
	return r
}

func logsFor(t *testing.T, msg payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) stubRepo {
	t.Helper()
	return stubRepo{logs: []*types.Log{encodeLog(t, teeABI(t), testOpType, op.Pay, msg)}}
}

// --- tests -----------------------------------------------------------------

func TestVerify_Success(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 150000, uint64(7)
	const batchID = uint64(900000)

	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:           testTxid,
			BlockNumber:    800000,
			BlockTimestamp: 1700000000,
			Fee:            1234,
			Outputs:        batchOutputs(t, nonce, out(recipScript, amount), out([]byte{0x52}, 250)), // recipient + change
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)

	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(0), resp.TransactionStatus)
	require.Equal(t, recipAddr, resp.RecipientAddress)
	require.Equal(t, big.NewInt(amount), resp.Amount)
	require.Equal(t, big.NewInt(amount), resp.ReceivedAmount)
	require.Equal(t, big.NewInt(1234), resp.TransactionFee)
	require.Equal(t, uint64(800000), resp.BlockNumber)
	require.Equal(t, uint64(1700000000), resp.BlockTimestamp)
	require.Equal(t, testTxid, hex.EncodeToString(resp.TransactionId[:]))
}

// The request's txid must reach the source. If it were dropped, the node path
// would have nothing to look up and would silently fall back to searching.
func TestVerify_RequestTxidReachesTheSource(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 150000, uint64(7)
	const batchID = uint64(900000)

	var got locator
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, gotLocator: &got, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, nonce, out(recipScript, amount)),
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)

	_, err := v.Verify(context.Background(), reqWithTxid(t, batchID, testTxid))
	require.NoError(t, err)
	require.Equal(t, testTxid, got.TransactionID, "the locator must carry the request's txid")
	require.Equal(t, testAnchor, got.AnchorAddress)
	require.Equal(t, nonce, got.Nonce)
}

// The binding that makes a supplied txid safe: a transaction whose input[0]
// does not spend this chain's anchor is refused, however well-formed it is.
// Only k-of-n can spend that anchor, so this is what an outsider cannot forge —
// the nonce OP_RETURN alone is free to write.
func TestVerify_WrongAnchorChainFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 150000, uint64(7)
	const batchID = uint64(900000)

	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, nonce, out(recipScript, amount)),
			// A real transaction, correct nonce, but it continues someone
			// else's chain.
			AnchorInputAddress: "bc1qsomeoneelse",
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)

	_, err := v.Verify(context.Background(), reqWithTxid(t, batchID, testTxid))
	require.Error(t, err)
	require.ErrorContains(t, err, "not anchor chain")
}

// A source that reports what input[0] spent, and reports the right thing, must
// still pass — the check is a binding, not a blanket refusal.
func TestVerify_MatchingAnchorChainPasses(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 150000, uint64(7)
	const batchID = uint64(900000)

	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:               testTxid,
			Outputs:            batchOutputs(t, nonce, out(recipScript, amount)),
			AnchorInputAddress: testAnchor,
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)

	resp, err := v.Verify(context.Background(), reqWithTxid(t, batchID, testTxid))
	require.NoError(t, err)
	require.Equal(t, uint8(0), resp.TransactionStatus)
}

// A CSP request with no locator is malformed, not a payment that did not
// happen: answering "not found" would let an omission read as evidence.
func TestNodeSourceRefusesMissingTxid(t *testing.T) {
	_, err := nodeSource{}.Batch(context.Background(), locator{AnchorAddress: testAnchor, Nonce: 7})
	require.ErrorIs(t, err, ErrMissingTransactionID)
}

func TestVerify_Undeliverable(t *testing.T) {
	recipAddr, _ := recipient(t)
	const nonce = uint64(7)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 500, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, nonce, out([]byte{0x52}, 250)), // only change, no recipient output
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(1), resp.TransactionStatus)
	require.Equal(t, "sub_dust_amount", resp.RevertReason)
	require.Equal(t, big.NewInt(0), resp.ReceivedAmount)
}

func TestVerify_NotFoundWhenAnchorUnresolved(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorAddr: ""}, // anchor output not indexed yet
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(2), resp.TransactionStatus)
	require.Equal(t, recipAddr, resp.RecipientAddress)
}

func TestVerify_NotFoundWhenBatchAbsent(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: nil}, // not confirmed/indexed
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(2), resp.TransactionStatus)
}

func TestVerify_ResolverErrorPropagates(t *testing.T) {
	v := newVerifier(t, stubRepo{}, stubIndexer{}, stubResolver{err: paymentdb.ErrDatabase})
	_, err := v.Verify(context.Background(), req(5))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_AmountMismatchFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const nonce = uint64(7)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 150000, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, nonce, out(recipScript, 149999)), // one sat short
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_NonceMismatchFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, 8, out(recipScript, 1000)), // tx nonce 8 != instruction nonce 7
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_PaymentNotInBatchNotFound(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t,
		logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)), // carries paymentId 900000
		stubIndexer{anchorAddr: testAnchor},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)},
	)
	_, err := v.Verify(context.Background(), req(batchID+1)) // ask for 900001
	require.ErrorIs(t, err, paymentdb.ErrRecordNotFound)
}

func TestVerify_RepoErrorPropagates(t *testing.T) {
	const batchID = uint64(900000)
	v := newVerifier(t, stubRepo{err: paymentdb.ErrDatabase}, stubIndexer{},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_AnchorResolveErrorPropagates(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t, logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{}, stubResolver{id: batchID, anchorErr: paymentdb.ErrDatabase})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_AnchorAddressErrorPropagates(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t, logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorErr: paymentdb.ErrDatabase}, stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_BatchErrorPropagates(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	v := newVerifier(t, logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorAddr: testAnchor, batchErr: paymentdb.ErrDatabase},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_MalformedBatchFailsClosed(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	// Outputs without a valid FLR nonce OP_RETURN at index 2.
	outs := []batchtx.Output{out([]byte{0x51}, 1000), out([]byte{0x51}, 330), out([]byte{0x51}, 0)}
	v := newVerifier(t, logsFor(t, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: outs}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_GroupOutOfRangeFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const batchID = uint64(900000)
	// paymentId is 5 past the batch start but the batch has a single group.
	msg := sampleMsg(recipAddr, 1000, batchID+5, batchID, 7, 1)
	v := newVerifier(t, logsFor(t, msg),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, 7, out(recipScript, 1000))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID+5))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_InvalidRecipientAddressFailsClosed(t *testing.T) {
	const batchID = uint64(900000)
	msg := sampleMsg("not-a-valid-address", 1000, batchID, batchID, 7, 1)
	v := newVerifier(t, logsFor(t, msg),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, 7, out([]byte{0x52}, 1000))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_AmountOverflowFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const batchID = uint64(900000)
	msg := sampleMsg(recipAddr, 0, batchID, batchID, 7, 1)
	msg.Amount = new(big.Int).Lsh(big.NewInt(1), 64) // > MaxInt64
	v := newVerifier(t, logsFor(t, msg),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, 7, out(recipScript, 1000))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_ConsistencyMismatchesFailClosed(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	cases := map[string]func(*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage){
		"sourceId": func(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) {
			m.SourceId = common.HexToHash("0xdead")
		},
		"account":        func(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) { m.AccountAddress = "someone-else" },
		"batchPaymentId": func(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) { m.BatchPaymentId = batchID + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			msg := sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)
			mutate(&msg)
			v := newVerifier(t, logsFor(t, msg), stubIndexer{}, stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
			_, err := v.Verify(context.Background(), req(batchID))
			require.ErrorIs(t, err, paymentdb.ErrDatabase)
		})
	}
}

// TestVerify_ReissuedPaymentIsNotADuplicate: a reissued batch is dispatched
// again, so every payment in it is emitted once per attempt. Both attempts
// respend the same anchor outpoint at the same nonce, so the repeats describe
// the same payment — treating them as an inconsistency made every payment that
// was ever reissued permanently unprovable.
func TestVerify_ReissuedPaymentIsNotADuplicate(t *testing.T) {
	recipAddr, script := recipient(t)
	const batchID = uint64(900000)
	msg := sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)
	log := encodeLog(t, teeABI(t), testOpType, op.Pay, msg)
	v := newVerifier(t, stubRepo{logs: []*types.Log{log, log}},
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{
			Txid:    testTxid,
			Outputs: batchOutputs(t, 7, out(script, 1000)),
		}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	res, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, uint8(0), res.TransactionStatus)
}

// TestVerify_ConflictingPaymentFailsClosed: repeats that DISAGREE are still an
// index inconsistency, and must not be answered.
func TestVerify_ConflictingPaymentFailsClosed(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	a := encodeLog(t, teeABI(t), testOpType, op.Pay, sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1))
	b := encodeLog(t, teeABI(t), testOpType, op.Pay, sampleMsg(recipAddr, 2000, batchID, batchID, 7, 1))
	v := newVerifier(t, stubRepo{logs: []*types.Log{a, b}}, stubIndexer{},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_DecodeErrorPropagates(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	// Log encoded with a different opType than the request -> envelope op mismatch.
	msg := sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)
	badLog := encodeLog(t, teeABI(t), common.HexToHash("0xbeef"), op.Pay, msg)
	v := newVerifier(t, stubRepo{logs: []*types.Log{badLog}}, stubIndexer{},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestNewBtcVerifier_UnsupportedSource(t *testing.T) {
	cfg := &config.PMWPaymentStatusConfig{}
	cfg.SourceIDPair = config.SourceIDEncodedPair{SourceID: config.SourceXRP}
	_, err := NewBtcVerifier(cfg, nil)
	require.ErrorIs(t, err, ErrUnsupportedSource)
}

// TestNewBtcVerifier_RequiresChannel: the node path is the only supported mode,
// so a BTC config without CHANNEL_ADDRESS fails closed rather than falling back
// to the (unimplemented) indexer path.
func TestNewBtcVerifier_RequiresChannel(t *testing.T) {
	cfg := &config.PMWPaymentStatusConfig{}
	cfg.SourceIDPair = config.SourceIDEncodedPair{SourceID: config.SourceTestBTC}
	_, err := NewBtcVerifier(cfg, nil)
	require.ErrorIs(t, err, ErrUnsupportedSource)
}

func TestNewBtcVerifier_OK(t *testing.T) {
	cfg := &config.PMWPaymentStatusConfig{}
	cfg.SourceIDPair = config.SourceIDEncodedPair{SourceID: config.SourceTestBTC}
	cfg.FlareRPCURL = "http://127.0.0.1:1" // dial is lazy; not contacted here
	cfg.TeePaymentsContractAddress = common.HexToAddress("0x1")
	cfg.ChannelAddress = common.HexToAddress("0x2") // node mode
	cfg.SourceRPCURL = "http://127.0.0.1:1"
	cfg.MinConfirmations = config.DefaultBtcMinConfirmations
	v, err := NewBtcVerifier(cfg, nil)
	require.NoError(t, err)
	require.NoError(t, v.Close()) // real OnChainResolver -> exercises its io.Closer
}

func TestVerify_InvalidTxidFailsClosed(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 1000, uint64(7)
	const batchID = uint64(900000)
	v := newVerifier(t, logsFor(t, sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: "zz", Outputs: batchOutputs(t, nonce, out(recipScript, amount))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestVerify_SuccessNilTokenId(t *testing.T) {
	recipAddr, recipScript := recipient(t)
	const amount, nonce = 1000, uint64(7)
	const batchID = uint64(900000)
	msg := sampleMsg(recipAddr, amount, batchID, batchID, nonce, 1)
	msg.TokenId = nil // exercises the nil-token normalization in baseResponse
	v := newVerifier(t, logsFor(t, msg),
		stubIndexer{anchorAddr: testAnchor, batch: &batchtx.BatchTx{Txid: testTxid, Outputs: batchOutputs(t, nonce, out(recipScript, amount))}},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	resp, err := v.Verify(context.Background(), req(batchID))
	require.NoError(t, err)
	require.Equal(t, []byte{}, resp.TokenId)
}

func TestBtcVerifier_Close(t *testing.T) {
	// Resolver stub is not an io.Closer, so Close is a no-op returning nil.
	v := newVerifier(t, stubRepo{}, stubIndexer{}, stubResolver{})
	require.NoError(t, v.Close())
}

func TestNetworkParamsFollowTheSourceAndTheOverride(t *testing.T) {
	// The source id names the chain; the network is a deployment fact.
	p, err := config.BtcNetworkParams(config.SourceBTC, "")
	require.NoError(t, err)
	require.Equal(t, &chaincfg.MainNetParams, p)

	p, err = config.BtcNetworkParams(config.SourceTestBTC, "")
	require.NoError(t, err)
	require.Equal(t, &chaincfg.SigNetParams, p)

	// Regtest cannot be inferred from any source id, which is exactly why the
	// override exists: without it a regtest deployment encodes bc1... addresses
	// and every comparison silently misses.
	p, err = config.BtcNetworkParams(config.SourceBTC, "regtest")
	require.NoError(t, err)
	require.Equal(t, &chaincfg.RegressionNetParams, p)

	_, err = config.BtcNetworkParams(config.SourceXRP, "")
	require.Error(t, err)

	_, err = config.BtcNetworkParams(config.SourceBTC, "moonnet")
	require.Error(t, err)
}

func TestTxidToBytes32(t *testing.T) {
	valid := strings.Repeat("ab", 32)
	b, err := txidToBytes32(valid)
	require.NoError(t, err)
	require.Equal(t, valid, hex.EncodeToString(b[:]))

	_, err = txidToBytes32("abcd")
	require.Error(t, err)

	_, err = txidToBytes32(strings.Repeat("zz", 32))
	require.Error(t, err)
}

func TestSatoshis(t *testing.T) {
	got, err := satoshis(big.NewInt(21_000_000))
	require.NoError(t, err)
	require.Equal(t, int64(21_000_000), got)

	_, err = satoshis(big.NewInt(-1))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)

	_, err = satoshis(nil)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)

	tooBig := new(big.Int).Lsh(big.NewInt(1), 64)
	_, err = satoshis(tooBig)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

// TestVerify_NonPayCommandIsIgnored: the decoder filters on the command as well
// as the opType, and a batch's per-payment records are PAY. A REISSUE event
// sharing the instruction id must not be mistaken for one of them.
func TestVerify_NonPayCommandIsIgnored(t *testing.T) {
	recipAddr, _ := recipient(t)
	const batchID = uint64(900000)
	msg := sampleMsg(recipAddr, 1000, batchID, batchID, 7, 1)
	log := encodeLog(t, teeABI(t), testOpType, op.Reissue, msg)
	v := newVerifier(t, stubRepo{logs: []*types.Log{log}}, stubIndexer{},
		stubResolver{id: batchID, anchorTxid: strings.Repeat("cd", 32)})
	_, err := v.Verify(context.Background(), req(batchID))
	require.Error(t, err)
}
