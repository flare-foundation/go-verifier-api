package attestationtypes

import (
	"github.com/ethereum/go-ethereum/common"
)

type PMWMultisigAccountHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x504d574d756c74697369674163636f756e74436f6e6669677572656400000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7465737478727000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type PMWMultisigAccountEncodedRequest struct {
	FTDCHeader  PMWMultisigAccountHeader
	RequestBody string `json:"requestBody"`
}
type PMWMultisigAccountRequest struct {
	FTDCHeader  PMWMultisigAccountHeader      `json:"header"`
	RequestData PMWMultisigAccountRequestBody `json:"requestData"`
}

type PMWMultisigAccountRequestBody struct {
	WalletAddress string   `json:"walletAddress" validate:"required,hash32" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` // TODO
	PublicKeys    []string `json:"publicKeys" validate:"required" example:"1"`                                                                            // TODO
	Threshold     uint64   `json:"threshold" validate:"required" example:"1"`                                                                             // TODO
}

type PMWMultisigAccountRequestData struct {
	WalletAddress string
	PublicKeys    []common.Hash // TODO 32 byte or any
	Threshold     uint64
}

func (requestBody PMWMultisigAccountRequestBody) ToInternal() (PMWMultisigAccountRequestData, error) {
	var hashes []common.Hash
	for _, pk := range requestBody.PublicKeys {
		hashes = append(hashes, common.HexToHash(pk))
	}

	return PMWMultisigAccountRequestData{
		WalletAddress: requestBody.WalletAddress,
		PublicKeys:    hashes,
		Threshold:     requestBody.Threshold,
	}, nil
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

type PMWMultisigAccountResponseData struct {
	PMWMultisigAccountStatus uint8
	Sequence                 uint64
}

func (data PMWMultisigAccountResponseData) ToExternal() PMWMultisigAccountResponseBody {
	return PMWMultisigAccountResponseBody{
		PMWMultisigAccountStatus: data.PMWMultisigAccountStatus,
		Sequence:                 data.Sequence,
	}
}

type RawAndEncodedPMWMultisigAccountResponseBody struct {
	ResponseData PMWMultisigAccountResponseBody `json:"responseData"`
	ResponseBody string                         `json:"responseBody" example:"0x0000abcd..."`
}
