package attestationtypes

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

type TeeAvailabilityResponseData struct {
	Status                 uint8       `json:"status"`
	TeeTimestamp           uint64      `json:"teeTimestamp"`
	CodeHash               common.Hash `json:"codeHash"`
	Platform               common.Hash `json:"platform"`
	InitialSigningPolicyId uint32      `json:"initialSigningPolicyId"`
	LastSigningPolicyId    uint32      `json:"lastSigningPolicyId"`
	StateHash              common.Hash `json:"stateHash"`
}

type RawAndEncodedResponseBody struct {
	ResponseBody        TeeAvailabilityResponseData `json:"responseBody"`
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
