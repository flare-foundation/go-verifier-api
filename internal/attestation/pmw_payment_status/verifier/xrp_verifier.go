package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/events"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/models"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type XRPVerifier struct {
	db       *gorm.DB
	cChainDb *gorm.DB
	config   *pmwpaymentstatusconfig.PMWPaymentStatusConfig
}

type chainQuery struct {
	SourceAddress string
	Nonce         uint64
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWPaymentStatusRequestBody) (connector.IPMWPaymentStatusResponseBody, error) {
	// Build instruction Id
	sourceEnv := string(x.config.SourcePair.SourceId)
	instructionId := GenerateInstructionId(req.WalletId, req.Nonce, sourceEnv)
	// Query event
	chainLog, err := x.fetchInstructionLog(ctx, x.cChainDb, instructionId)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Decode event data
	paymentMessage, err := DecodeTeeInstructionsSentEventData(chainLog)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Query underlying chain for transaction
	dbTransaction, err := x.getTransactionBySourceAndSequence(ctx, x.db, chainQuery{paymentMessage.SenderAddress, req.Nonce})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("transaction not found")
		}
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Parse transaction response JSON into structured data
	rawTransactionData, err := x.parseRawTransactionData(dbTransaction.Response)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Validate transaction and build response
	resp, err := x.buildPaymentStatusResponse(rawTransactionData, paymentMessage, dbTransaction)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	return resp, nil
}

func (x *XRPVerifier) fetchInstructionLog(ctx context.Context, db *gorm.DB, instructionId string) (*types.Log, error) {
	var dbLog database.Log
	teeInstructionsSentEventHash, e := GetTeeInstructionsSentEventSignature()
	if e != nil {
		return nil, e
	}
	err := db.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ?", teeInstructionsSentEventHash, instructionId).
		First(&dbLog).Error
	if err != nil {
		return nil, fmt.Errorf("log not found for instruction %s", instructionId)
	}
	log, err := events.ConvertDatabaseLogToChainLog(dbLog)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func (x *XRPVerifier) getTransactionBySourceAndSequence(ctx context.Context, db *gorm.DB, query chainQuery) (models.DBTransaction, error) {
	var tx models.DBTransaction
	err := db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		First(&tx).Error
	if err != nil {
		return models.DBTransaction{}, err
	}
	return tx, nil
}

func (x *XRPVerifier) parseRawTransactionData(response string) (RawTransactionData, error) {
	var rawTransactionData RawTransactionData
	err := json.Unmarshal([]byte(response), &rawTransactionData)
	if err != nil {
		logger.Errorf("failed to unmarshal XRP transaction response: %v, response: %s", err, response)
		return rawTransactionData, err
	}
	// Validate required fields // TODO-later
	if rawTransactionData.MetaData.TransactionResult == "" {
		return rawTransactionData, fmt.Errorf("missing transaction result in raw transaction data")
	}
	return rawTransactionData, nil
}

func (x *XRPVerifier) buildPaymentStatusResponse(raw RawTransactionData, paymentMsg *payment.ITeePaymentsPaymentInstructionMessage, tx models.DBTransaction) (connector.IPMWPaymentStatusResponseBody, error) {
	transactionResult, err := GetTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	transactionFee, err := NewBigIntFromString(raw.Fee)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	hashBytes, err := HexStringToBytes32(tx.Hash)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	receivedAmount, err := FindReceivedAmountForAddress(&raw.MetaData, paymentMsg.RecipientAddress)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	return connector.IPMWPaymentStatusResponseBody{
		TransactionStatus: uint8(transactionResult),
		SenderAddress:     GetStandardAddressHash(paymentMsg.SenderAddress),
		RecipientAddress:  GetStandardAddressHash(paymentMsg.RecipientAddress),
		Amount:            paymentMsg.Amount,
		PaymentReference:  paymentMsg.PaymentReference,
		ReceivedAmount:    receivedAmount,
		TransactionFee:    transactionFee,
		TransactionId:     hashBytes,
		BlockNumber:       tx.BlockNumber,
		BlockTimestamp:    tx.Timestamp,
	}, nil
}
