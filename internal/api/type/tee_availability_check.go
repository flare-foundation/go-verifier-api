package attestationtypes

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type TeeAvailabilityEncodedRequest struct {
	AttestationType string `json:"attestationType" validate:"required,hash32" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000"`
	SourceId        string `json:"sourceId" validate:"required,hash32" example:"0x7465650000000000000000000000000000000000000000000000000000000000"`
	RequestBody     string `json:"requestBody" example:"0x000000000000000000000000000000000000000000000000000000000000dead0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000ab54a98ceb1f0ad2000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"`
}

type TeeAvailabilityRequest struct {
	AttestationType string                     `json:"attestationType" validate:"required,hash32" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000"`
	SourceId        string                     `json:"sourceId" validate:"required,hash32" example:"0x7465650000000000000000000000000000000000000000000000000000000000"`
	RequestBody     TeeAvailabilityRequestBody `json:"requestBody"`
}

type TeeAvailabilityRequestBody struct {
	TeeId     string `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge string `json:"challenge" validate:"required,numeric" example:"12345678901234567890"`
}

type TeeAvailabilityRequestData struct {
	TeeId     common.Address
	Url       string
	Challenge *big.Int
}

func (requestBody TeeAvailabilityRequestBody) ToInternal() (TeeAvailabilityRequestData, error) {
	addr := common.HexToAddress(requestBody.TeeId)

	challenge := new(big.Int)
	if _, ok := challenge.SetString(requestBody.Challenge, 10); !ok {
		return TeeAvailabilityRequestData{}, fmt.Errorf("invalid challenge value: %s", requestBody.Challenge)
	}

	return TeeAvailabilityRequestData{
		TeeId:     addr,
		Url:       requestBody.Url,
		Challenge: challenge,
	}, nil
}

type TeeAvailabilityResponseBody struct {
	Status                 uint8    `json:"status"`
	TeeTimestamp           uint64   `json:"teeTimestamp"`
	CodeHash               string   `json:"codeHash"`
	Platform               string   `json:"platform"`
	InitialSigningPolicyId *big.Int `json:"initialSigningPolicyId"` // TODO for type
	LastSigningPolicyId    *big.Int `json:"lastSigningPolicyId"`    // TODO for type
	StateHash              [32]byte `json:"stateHash"`              // TODO for type
}

type TeeAvailabilityResponseData struct {
	Status                 uint8
	TeeTimestamp           uint64
	CodeHash               [32]byte
	Platform               [32]byte
	InitialSigningPolicyId *big.Int
	LastSigningPolicyId    *big.Int
	StateHash              [32]byte
}

func (internal TeeAvailabilityResponseData) FromInternal() TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:                 internal.Status,
		TeeTimestamp:           internal.TeeTimestamp,
		CodeHash:               hex.EncodeToString(internal.CodeHash[:]),
		Platform:               hex.EncodeToString(internal.Platform[:]),
		InitialSigningPolicyId: internal.InitialSigningPolicyId,
		LastSigningPolicyId:    internal.LastSigningPolicyId,
		StateHash:              internal.StateHash,
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
	TeeInfo         ProxyInfoData   `json:"teeInfo"`     //TODO: ProxyInfoData TBD
	State           string          `json:"state"`       //TODO: ProxyInfoData TBD
	Version         string          `json:"version"`     //TODO: ProxyInfoData TBD
	AttestationInfo AttestationInfo `json:"attestation"` //TODO: AttestationInfo TBD
}

type AttestationInfo struct {
	Platform    string `json:"platform"`
	Attestation string `json:"attestation"`
}

type ProxyInfoData struct {
	Challenge                string          `json:"challenge"`
	PublicKey                ecdsa.PublicKey `json:"publicKey"`
	InitialSigningPolicyId   *big.Int        `json:"initialSigningPolicyId"`
	InitialSigningPolicyHash common.Hash     `json:"initialSigningPolicyHash"`
	LastSigningPolicyId      *big.Int        `json:"lastSigningPolicyId"`
	LastSigningPolicyHash    common.Hash     `json:"lastSigningPolicyHash"`
	StateHash                common.Hash     `json:"stateHash"`
	TeeTimestamp             uint64          `json:"teeTimestamp"`
}
