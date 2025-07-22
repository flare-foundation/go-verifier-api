package attestationtypes

type PMWPaymentStatusHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x504d575061796d656e7453746174757300000000000000000000000000000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7872700000000000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type PMWPaymentStatusEncodedRequest struct {
	Header      PMWPaymentStatusHeader
	RequestBody string `json:"requestBody"`
}
type PMWPaymentStatusRequest struct {
	Header      PMWPaymentStatusHeader      `json:"header"`
	RequestBody PMWPaymentStatusRequestBody `json:"requestBody"`
}

type PMWPaymentStatusRequestBody struct {
	WalletId string `json:"walletId" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Nonce    uint64 `json:"nonce" validate:"required" example:"1"`
	SubNonce uint64 `json:"subNonce" validate:"required" example:"1"`
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
