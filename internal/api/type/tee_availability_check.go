package attestationtypes

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type TeeAvailabilityHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7465650000000000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type TeeAvailabilityEncodedRequest struct {
	Header      TeeAvailabilityHeader
	RequestBody string `json:"requestBody" example:"0x000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"`
}
type TeeAvailabilityRequest struct {
	Header      TeeAvailabilityHeader      `json:"header"`
	RequestBody TeeAvailabilityRequestBody `json:"requestBody"`
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
	InitialSigningPolicyId json.Number `json:"initialSigningPolicyId"` // TODO for type
	LastSigningPolicyId    json.Number `json:"lastSigningPolicyId"`    // TODO for type
	StateHash              string      `json:"stateHash"`              // TODO for type
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

type RawAndEncodedResponseBody struct {
	ResponseBody        TeeAvailabilityResponseBody `json:"responseBody"`
	EncodedResponseBody string                      `json:"encodedResponseBody" example:"0x0000abcd..."`
}

type AvailabilityCheckStatus uint8

// Match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L12
const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)

type ProxyInfoResponseBody struct {
	TeeInfo     ProxyInfoData `json:"teeInfo"`
	State       string        `json:"state"`
	Version     string        `json:"version"`
	Attestation string        `json:"attestation"`
	Platform    string        `json:"platform"`
}

type ProxyInfoData struct {
	Challenge                common.Hash     `json:"challenge"` // TODO is this challengeInstructionId ?
	PublicKey                ecdsa.PublicKey `json:"publicKey"`
	InitialSigningPolicyId   uint32          `json:"initialSigningPolicyId"`
	InitialSigningPolicyHash common.Hash     `json:"initialSigningPolicyHash"`
	LastSigningPolicyId      uint32          `json:"lastSigningPolicyId"`
	LastSigningPolicyHash    common.Hash     `json:"lastSigningPolicyHash"`
	StateHash                common.Hash     `json:"stateHash"`
	TeeTimestamp             uint64          `json:"teeTimestamp"`
}

func (teeInfo ProxyInfoData) Hash() (common.Hash, error) {
	encoded, err := json.Marshal(teeInfo)
	if err != nil {
		return common.Hash{}, err
	}
	hash := crypto.Keccak256(encoded)
	var res common.Hash
	copy(res[:], hash)

	return res, nil
}
