package attestationtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type PMWPaymentStatusRequest = FTDCRequest[PMWPaymentStatusRequestBody]

type PMWPaymentStatusRequestBody struct {
	WalletId string `json:"walletId" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Nonce    uint64 `json:"nonce" validate:"required" example:"1"`
	SubNonce uint64 `json:"subNonce" validate:"required" example:"1"`
}

func (requestBody PMWPaymentStatusRequestBody) ToInternal() (connector.IPMWPaymentStatusRequestBody, error) {
	return connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash(requestBody.WalletId),
		Nonce:    requestBody.Nonce,
		SubNonce: requestBody.SubNonce,
	}, nil
}

type PMWPaymentStatusResponseBody struct {
	SenderAddress     string `json:"senderAddress"`
	RecipientAddress  string `json:"recipientAddress"`
	Amount            string `json:"amount"`
	Fee               string `json:"fee"`
	PaymentReference  string `json:"paymentReference"`
	TransactionStatus uint8  `json:"transactionStatus"`
	RevertReason      string `json:"revertReason"`
	ReceivedAmount    string `json:"receivedAmount"`
	TransactionFee    string `json:"transactionFee"`
	TransactionId     string `json:"transactionId"`
	BlockNumber       uint64 `json:"blockNumber"`
	BlockTimestamp    uint64 `json:"blockTimestamp"`
}

func PMWPaymentToExternal(data connector.IPMWPaymentStatusResponseBody) PMWPaymentStatusResponseBody {
	return PMWPaymentStatusResponseBody{
		SenderAddress:     data.SenderAddress,
		RecipientAddress:  data.RecipientAddress,
		Amount:            data.Amount.String(),
		Fee:               data.Fee.String(),
		PaymentReference:  common.BytesToHash(data.PaymentReference[:]).Hex(),
		TransactionStatus: data.TransactionStatus,
		RevertReason:      data.RevertReason,
		ReceivedAmount:    data.ReceivedAmount.String(),
		TransactionFee:    data.TransactionFee.String(),
		TransactionId:     common.BytesToHash(data.TransactionId[:]).Hex(),
		BlockNumber:       data.BlockNumber,
		BlockTimestamp:    data.BlockTimestamp,
	}
}

type RawAndEncodedPMWPaymentStatusResponseBody = RawAndEncodedFTDCResponse[PMWPaymentStatusResponseBody]
