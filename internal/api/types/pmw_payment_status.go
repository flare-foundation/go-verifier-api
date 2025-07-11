package attestationtypes

type AttestationRequestPMWPaymentStatus struct {
	AttestationType string                       `json:"attestationType" example:"0x4a736f6e41706900000000000000000000000000000000000000000000000000" validate:"required,hash32"`
	SourceID        string                       `json:"sourceId" example:"0x4a736f6e41706900000000000000000000000000000000000000000000000000" validate:"required,hash32"`
	RequestBody     IPMWPaymentStatusRequestBody `json:"requestBody" validate:"required"`
}

type FullAttestationResponsePMWPaymentStatus struct {
	AttestationStatus string `json:"attestationStatus"`
	Response          struct {
		AttestationType string                        `json:"attestationType"`
		SourceID        string                        `json:"sourceId"`
		RequestBody     IPMWPaymentStatusRequestBody  `json:"requestBody"`
		ResponseBody    IPMWPaymentStatusResponseBody `json:"responseBody"`
	} `json:"response"`
}

// copied from connector.IPMWPaymentStatusRequestBody
type IPMWPaymentStatusRequestBody struct {
	WalletId string `json:"walletId" validate:"required,hash32"`
	Nonce    uint64 `json:"nonce" validate:"required"`
	SubNonce uint64 `json:"subNonce" validate:"required"`
}

// copied from connector.IPMWPaymentStatusResponseBody
type IPMWPaymentStatusResponseBody struct {
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
