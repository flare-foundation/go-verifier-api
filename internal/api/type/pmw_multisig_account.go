package attestationtypes

type PMWMultisigAccountHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x504d575061796d656e7453746174757300000000000000000000000000000000"` //TODO
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7872700000000000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type PMWMultisigAccountEncodedRequest struct {
	Header      PMWMultisigAccountHeader
	RequestBody string `json:"requestBody"`
}
type PMWMultisigAccountRequest struct {
	Header      PMWMultisigAccountHeader      `json:"header"`
	RequestBody PMWMultisigAccountRequestBody `json:"requestBody"`
}

type PMWMultisigAccountRequestBody struct {
	WalletAddress string   `json:"walletAddress" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` // TODO
	PublicKeys    []string `json:"publicKeys" validate:"required" example:"1"`                                                                            // TODO
	Threshold     uint64   `json:"threshold" validate:"required" example:"1"`                                                                             // TODO
}

type PMWMultisigAccountResponseBody struct {
	PMWMultisigAccountStatus uint8  `json:"status"`
	Sequence                 uint64 `json:"sequence"`
}

type PMWMultisigAccountStatus int // TODO - from common?

const (
	PMWMultisigAccountStatusOK PMWMultisigAccountStatus = iota
	PMWMultisigAccountStatusERROR
)
