package attestationtypes

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type TeeAvailabilityHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7465650000000000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type TeeAvailabilityEncodedRequest struct {
	FTDCHeader  TeeAvailabilityHeader `json:"header"`
	RequestBody string                `json:"requestBody" example:"0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"`
}
type TeeAvailabilityRequest struct {
	FTDCHeader  TeeAvailabilityHeader      `json:"header"`
	RequestData TeeAvailabilityRequestBody `json:"requestData"`
}

type TeeAvailabilityRequestBody struct {
	TeeId     string `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge string `json:"challenge" validate:"required,hash32" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

type TeeAvailabilityRequestData struct {
	TeeId     common.Address
	Url       string
	Challenge common.Hash
}

func (requestBody TeeAvailabilityRequestBody) ToInternal() (TeeAvailabilityRequestData, error) {
	return TeeAvailabilityRequestData{
		TeeId:     common.HexToAddress(requestBody.TeeId),
		Url:       requestBody.Url,
		Challenge: common.HexToHash(requestBody.Challenge),
	}, nil
}

type TeeAvailabilityResponseBody struct {
	Status                 json.Number `json:"status"`
	TeeTimestamp           json.Number `json:"teeTimestamp"`
	CodeHash               string      `json:"codeHash"`
	Platform               string      `json:"platform"`
	InitialSigningPolicyId json.Number `json:"initialSigningPolicyId"`
	LastSigningPolicyId    json.Number `json:"lastSigningPolicyId"`
	StateHash              string      `json:"stateHash"`
}
type TeeAvailabilityResponseData struct {
	Status                 uint8       `json:"status"`
	TeeTimestamp           uint64      `json:"teeTimestamp"`
	CodeHash               common.Hash `json:"codeHash"`
	Platform               common.Hash `json:"platform"`
	InitialSigningPolicyId uint32      `json:"initialSigningPolicyId"`
	LastSigningPolicyId    uint32      `json:"lastSigningPolicyId"`
	StateHash              common.Hash `json:"stateHash"`
}

func (data TeeAvailabilityResponseData) ToExternal() TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:                 json.Number(fmt.Sprintf("%d", data.Status)),
		TeeTimestamp:           json.Number(fmt.Sprintf("%d", data.TeeTimestamp)),
		CodeHash:               data.CodeHash.Hex(),
		Platform:               data.Platform.Hex(),
		InitialSigningPolicyId: json.Number(fmt.Sprintf("%d", data.InitialSigningPolicyId)),
		LastSigningPolicyId:    json.Number(fmt.Sprintf("%d", data.LastSigningPolicyId)),
		StateHash:              data.StateHash.Hex(),
	}
}

type RawAndEncodedTeeAvailabilityResponseBody struct {
	ResponseData TeeAvailabilityResponseBody `json:"responseData"`
	ResponseBody string                      `json:"responseBody" example:"0x0000abcd..."`
}

type AvailabilityCheckStatus uint8

// Should match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L12
const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)
