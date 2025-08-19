package attestationtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type PMWPaymentStatusHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x504d575061796d656e7453746174757300000000000000000000000000000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7465737478727000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type PMWPaymentStatusEncodedRequest struct {
	FTDCHeader  PMWPaymentStatusHeader
	RequestBody string `json:"requestBody" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001"`
}
type PMWPaymentStatusRequest struct {
	FTDCHeader  PMWPaymentStatusHeader      `json:"header"`
	RequestData PMWPaymentStatusRequestBody `json:"requestData"`
}

type PMWPaymentStatusRequestBody struct {
	WalletId string `json:"walletId" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Nonce    uint64 `json:"nonce" validate:"required" example:"1"`
	SubNonce uint64 `json:"subNonce" validate:"required" example:"1"`
}

func (requestBody PMWPaymentStatusRequestBody) ToInternal() connector.IPMWPaymentStatusRequestBody {
	return connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash(requestBody.WalletId),
		Nonce:    requestBody.Nonce,
		SubNonce: requestBody.SubNonce,
	}
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

type RawAndEncodedPMWPaymentStatusResponseBody struct {
	ResponseData PMWPaymentStatusResponseBody `json:"responseData"`
	ResponseBody string                       `json:"responseBody" example:"0x0000abcd..."`
}
