package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
)

type PMWPaymentStatusRequestBody struct {
	OpType        common.Hash `json:"opType" validate:"required" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	SenderAddress string      `json:"senderAddress" validate:"required" example:"abcdef"`
	PaymentId     uint64      `json:"paymentId" validate:"required" example:"1"`
}

func (requestBody PMWPaymentStatusRequestBody) ToInternal() (fdc2.IPMWPaymentStatusRequestBody, error) {
	return fdc2.IPMWPaymentStatusRequestBody{
		OpType:        requestBody.OpType,
		SenderAddress: requestBody.SenderAddress,
		PaymentId:     requestBody.PaymentId,
	}, nil
}

type PMWPaymentStatusResponseBody struct {
	RecipientAddress  string        `json:"recipientAddress"`
	TokenID           hexutil.Bytes `json:"tokenId"`
	Amount            hexutil.Big   `json:"amount"`
	MaxFee            hexutil.Big   `json:"maxFee"`
	PaymentReference  common.Hash   `json:"paymentReference"`
	TransactionStatus uint8         `json:"transactionStatus"`
	RevertReason      string        `json:"revertReason"`
	ReceivedAmount    hexutil.Big   `json:"receivedAmount"`
	TransactionFee    hexutil.Big   `json:"transactionFee"`
	TransactionID     common.Hash   `json:"transactionId"`
	BlockNumber       uint64        `json:"blockNumber"`
	BlockTimestamp    uint64        `json:"blockTimestamp"`
}

func (s PMWPaymentStatusResponseBody) FromInternal(data fdc2.IPMWPaymentStatusResponseBody) ResponseConvertible[fdc2.IPMWPaymentStatusResponseBody] {
	return PMWPaymentStatusResponseBody{
		RecipientAddress:  data.RecipientAddress,
		TokenID:           data.TokenId,
		Amount:            hexutil.Big(*data.Amount),
		MaxFee:            hexutil.Big(*data.MaxFee),
		PaymentReference:  data.PaymentReference,
		TransactionStatus: data.TransactionStatus,
		RevertReason:      data.RevertReason,
		ReceivedAmount:    hexutil.Big(*data.ReceivedAmount),
		TransactionFee:    hexutil.Big(*data.TransactionFee),
		TransactionID:     data.TransactionId,
		BlockNumber:       data.BlockNumber,
		BlockTimestamp:    data.BlockTimestamp,
	}
}

func (s PMWPaymentStatusResponseBody) Log() {
	logger.Debugf("PMWPaymentStatus result: Recipient=%s, TokenID=%x, Amount=%v, MaxFee=%v, Reference=%x, Status=%d, Revert=%s, Received=%v, TxFee=%v, TxID=%x, Block=%d, Timestamp=%d",
		s.RecipientAddress,
		s.TokenID,
		s.Amount,
		s.MaxFee,
		s.PaymentReference,
		s.TransactionStatus,
		s.RevertReason,
		s.ReceivedAmount,
		s.TransactionFee,
		s.TransactionID,
		s.BlockNumber,
		s.BlockTimestamp,
	)
}

func LogPMWPaymentStatusRequestBody(req fdc2.IPMWPaymentStatusRequestBody) {
	logger.Debugf("PMWPaymentStatus request: OpType=%s, SenderAddress=%s, PaymentId=%d",
		convert.CommonHashToString(req.OpType), req.SenderAddress, req.PaymentId)
}
