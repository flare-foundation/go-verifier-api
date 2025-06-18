package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"gitlab.com/urskak/verifier-api/pkg/pmw_payment_status/config"
	"gitlab.com/urskak/verifier-api/pkg/pmw_payment_status/models"
	xrptypes "gitlab.com/urskak/verifier-api/pkg/pmw_payment_status/types"

	"github.com/flare-foundation/go-flare-common/pkg/events"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PaymentService struct {
	cChainDb *gorm.DB
	db       *gorm.DB
}

// Query for xrp indexer
type chainQuery struct {
	SourceAddress string
	Nonce         uint64
	SubNonce      uint64
}

func NewPaymentService() (*PaymentService, error) {
	dbURL, err := config.DatabaseURL()
	if err != nil {
		return nil, err
	}
	dbCChainURL, err := config.CchainDatabaseURL()
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	cChainDb, err := gorm.Open(mysql.Open(dbCChainURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	return &PaymentService{
		cChainDb: cChainDb,
		db:       db,
	}, nil
}

func (ps *PaymentService) fetchInstructionLog(ctx context.Context, db *gorm.DB, instructionId string) (*types.Log, error) {
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

func (ps *PaymentService) getTransactionByAddressAndNonce(ctx context.Context, db *gorm.DB, query chainQuery) (models.DBTransaction, error) {
	var tx models.DBTransaction
	err := db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		First(&tx).Error
	if err != nil {
		return models.DBTransaction{}, err
	}
	return tx, nil
}

func (ps *PaymentService) parseRawTransactionData(response string) (xrptypes.RawTransactionData, error) {
	var rawTransactionData xrptypes.RawTransactionData
	err := json.Unmarshal([]byte(response), &rawTransactionData)
	if err != nil {
		logger.Errorf("failed to unmarshal XRP transaction response: %v, response: %s", err, response)
		return rawTransactionData, err
	}
	// Validate required fields // TODO
	if rawTransactionData.MetaData.TransactionResult == "" {
		return rawTransactionData, fmt.Errorf("missing transaction result in raw transaction data")
	}
	return rawTransactionData, nil
}

func (ps *PaymentService) buildPaymentStatusResponse(raw xrptypes.RawTransactionData, paymentMsg *payment.ITeePaymentsPaymentInstructionMessage, tx models.DBTransaction) (connector.IPMWPaymentStatusResponseBody, error) {
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

func (ps *PaymentService) VerifyPMWPaymentStatus(ctx context.Context, req connector.IPMWPaymentStatusRequestBody) (connector.IPMWPaymentStatusResponseBody, error) {
	// Build instruction Id
	sourceEnv, err := config.SourceID()
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	instructionId := GenerateInstructionId(req.WalletId, req.Nonce, sourceEnv)
	// Query event (in case of batch payments (UTXO chains), subNonce needs to be used to fetch appropriate PaymentInstructionMessage)
	chainLog, err := ps.fetchInstructionLog(ctx, ps.cChainDb, instructionId)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Decode event data
	paymentMessage, err := DecodeTeeInstructionsSentEventData(chainLog)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Query underlying chain for transaction
	dbTransaction, err := ps.getTransactionByAddressAndNonce(ctx, ps.db, chainQuery{paymentMessage.SenderAddress, req.Nonce, req.SubNonce})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("transaction not found")
		}
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Parse transaction response JSON into structured data
	rawTransactionData, err := ps.parseRawTransactionData(dbTransaction.Response)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Validate transaction and build response
	resp, err := ps.buildPaymentStatusResponse(rawTransactionData, paymentMessage, dbTransaction)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	return resp, nil
}
