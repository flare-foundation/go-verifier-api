package btcverifier

// The two sources of a batch's per-payment instruction messages.
//
// A payment is proven against the batch that carried it, and to do that the
// verifier needs that payment's own terms — recipient, amount, reference — plus
// the batch's anchor and nonce. Both dispatch paths publish exactly that, one
// message per payment, all sharing the batch instruction id. They publish it
// from different contracts, in different events, for different reasons:
//
//	TeePaymentsUtxo  TeeInstructionsSent, from the TEE diamond, one per payment.
//	                 Each IS an instruction; a machine acts on it.
//
//	CSP channel      PaymentBatched, from the channel, one per payment, emitted
//	                 at settle. NOT instructions: CSP dispatches one batch
//	                 instruction to the machines, and a payment only acquires an
//	                 anchor, a nonce and a transaction when its batch settles —
//	                 which is why these are emitted then and not at pay().

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"

	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
)

// TeeInstructionMessages reads the per-payment messages from the diamond's
// TeeInstructionsSent events — the TeePaymentsUtxo path.
type TeeInstructionMessages struct {
	Repo logRepo
	ABI  abi.ABI
}

func (t TeeInstructionMessages) Messages(ctx context.Context, instructionID, opType common.Hash) (
	[]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error,
) {
	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(t.ABI)
	if err != nil {
		return nil, err
	}
	logs, err := t.Repo.FetchInstructionLogsForID(ctx, eventHash, instructionID)
	if err != nil {
		return nil, err
	}
	out := make([]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, 0, len(logs))
	for _, l := range logs {
		m, err := teeinstruction.DecodeUtxoTeeInstructionsSentEventData(l, t.ABI, op.Pay, opType)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// PaymentBatchedMessages reads the per-payment messages from the CSP channel's
// PaymentBatched events.
//
//	event PaymentBatched(bytes32 indexed instructionId, uint64 indexed paymentId, bytes message)
//
// `message` is abi.encode(UtxoPaymentInstructionMessage) — the same struct the
// diamond path carries — so only the envelope differs and the attestation is
// identical on both paths.
type PaymentBatchedMessages struct {
	Repo logRepo
	ABI  abi.ABI // must declare PaymentBatched
}

// PaymentBatchedEvent is the event name this source reads.
const PaymentBatchedEvent = "PaymentBatched"

// maxEventDataSize bounds a single event's data before ABI decoding, matching the
// diamond decoder's guard so a hostile row cannot drive unbounded work.
const maxEventDataSize = 1 << 20 // 1 MB

func (p PaymentBatchedMessages) Messages(ctx context.Context, instructionID, _ common.Hash) (
	[]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error,
) {
	ev, ok := p.ABI.Events[PaymentBatchedEvent]
	if !ok {
		return nil, fmt.Errorf("ABI declares no %s event: %w", PaymentBatchedEvent, paymentdb.ErrDatabase)
	}

	// PaymentBatched indexes the instruction id FIRST — there is no extension
	// id in front of it, as there is on the diamond's event.
	logs, err := p.Repo.FetchLogsByInstructionTopic1(ctx, ev.ID.Hex(), instructionID)
	if err != nil {
		return nil, err
	}

	out := make([]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, 0, len(logs))
	for _, l := range logs {
		// Bound the event data before decoding, matching the diamond decoder: a
		// hostile/corrupt row must not be ABI-decoded unbounded.
		if len(l.Data) > maxEventDataSize {
			return nil, fmt.Errorf("%s event data too large (%d bytes, max %d): %w",
				PaymentBatchedEvent, len(l.Data), maxEventDataSize, paymentdb.ErrDatabase)
		}
		// PaymentBatched indexes (instructionId, paymentId): topic[0] is the event
		// signature, topic[1] the instruction id, topic[2] the paymentId. The topic
		// is the indexer's authoritative paymentId, so bind the decoded message to
		// it — a decoded body disagreeing with its own topic is a corrupt row.
		if len(l.Topics) < 3 {
			return nil, fmt.Errorf("%s log has %d topics, expected at least 3: %w",
				PaymentBatchedEvent, len(l.Topics), paymentdb.ErrDatabase)
		}

		// The struct travels as a single `bytes` argument, so the event data is
		// an ABI-encoded bytes wrapping an ABI-encoded struct: unwrap, then
		// decode.
		unpacked, err := ev.Inputs.NonIndexed().Unpack(l.Data)
		if err != nil {
			return nil, fmt.Errorf("cannot unpack %s data: %v: %w", PaymentBatchedEvent, err, paymentdb.ErrDatabase)
		}
		if len(unpacked) != 1 {
			return nil, fmt.Errorf("%s carries %d non-indexed fields, expected 1: %w",
				PaymentBatchedEvent, len(unpacked), paymentdb.ErrDatabase)
		}
		raw, ok := unpacked[0].([]byte)
		if !ok {
			return nil, fmt.Errorf("%s message is not bytes: %w", PaymentBatchedEvent, paymentdb.ErrDatabase)
		}

		m, err := teeinstruction.DecodeUtxoPaymentInstructionMessage(raw)
		if err != nil {
			return nil, fmt.Errorf("cannot decode %s message: %v: %w", PaymentBatchedEvent, err, paymentdb.ErrDatabase)
		}
		topicPaymentID := l.Topics[2].Big()
		if !topicPaymentID.IsUint64() || topicPaymentID.Uint64() != m.PaymentId {
			return nil, fmt.Errorf("%s message paymentId %d disagrees with indexed topic %s: %w",
				PaymentBatchedEvent, m.PaymentId, l.Topics[2].Hex(), paymentdb.ErrDatabase)
		}
		out = append(out, m)
	}
	return out, nil
}

// paymentBatchedABI declares the one event this verifier reads from the CSP
// channel. Inlined rather than loaded from the channel's full ABI: the verifier
// needs exactly this event, and a minimal declaration cannot drift into
// depending on anything else the channel happens to expose.
const paymentBatchedABI = `[{"type":"event","name":"PaymentBatched","inputs":[
  {"name":"instructionId","type":"bytes32","indexed":true},
  {"name":"paymentId","type":"uint64","indexed":true},
  {"name":"message","type":"bytes","indexed":false}]}]`
