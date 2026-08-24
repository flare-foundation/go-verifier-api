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
