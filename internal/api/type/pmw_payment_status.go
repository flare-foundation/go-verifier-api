package attestationtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type PMWPaymentStatusRequestBody struct {
	WalletId common.Hash `json:"walletId" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Nonce    uint64      `json:"nonce" validate:"required" example:"1"`
	SubNonce uint64      `json:"subNonce" validate:"required" example:"1"`
}

func (requestBody PMWPaymentStatusRequestBody) ToInternal() (connector.IPMWPaymentStatusRequestBody, error) {
	return connector.IPMWPaymentStatusRequestBody{
		WalletId: requestBody.WalletId,
		Nonce:    requestBody.Nonce,
		SubNonce: requestBody.SubNonce,
	}, nil
}

type PMWPaymentStatusResponseBody struct {
	SenderAddress     string      `json:"senderAddress"`
	RecipientAddress  string      `json:"recipientAddress"`
	Amount            hexutil.Big `json:"amount"`
	Fee               hexutil.Big `json:"fee"`
	PaymentReference  common.Hash `json:"paymentReference"`
	TransactionStatus uint8       `json:"transactionStatus"`
	RevertReason      string      `json:"revertReason"`
	ReceivedAmount    hexutil.Big `json:"receivedAmount"`
	TransactionFee    hexutil.Big `json:"transactionFee"`
	TransactionId     common.Hash `json:"transactionId"`
	BlockNumber       uint64      `json:"blockNumber"`
	BlockTimestamp    uint64      `json:"blockTimestamp"`
}

func PMWPaymentToExternal(data connector.IPMWPaymentStatusResponseBody) PMWPaymentStatusResponseBody {
	return PMWPaymentStatusResponseBody{
		SenderAddress:     data.SenderAddress,
		RecipientAddress:  data.RecipientAddress,
		Amount:            hexutil.Big(*data.Amount),
		Fee:               hexutil.Big(*data.Fee),
		PaymentReference:  data.PaymentReference,
		TransactionStatus: data.TransactionStatus,
		RevertReason:      data.RevertReason,
		ReceivedAmount:    hexutil.Big(*data.ReceivedAmount),
		TransactionFee:    hexutil.Big(*data.TransactionFee),
		TransactionId:     data.TransactionId,
		BlockNumber:       data.BlockNumber,
		BlockTimestamp:    data.BlockTimestamp,
	}
}
