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
	Status        uint8  `json:"status"`
	MachineStatus uint8  `json:"machineStatus"`
	TeeTimestamp  uint64 `json:"teeTimestamp"`
	InitialTeeId  string `json:"initialTeeId"`
	CodeHash      string `json:"codeHash"`
	Platform      string `json:"platform"`
	RewardEpochId string `json:"rewardEpochId"`
}

type TeeAvailabilityResponseData struct {
	Status        uint8
	MachineStatus uint8
	TeeTimestamp  uint64
	InitialTeeId  common.Address
	CodeHash      [32]byte
	Platform      [32]byte
	RewardEpochId *big.Int
}

func (internal TeeAvailabilityResponseData) FromInternal() TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:        internal.Status,
		MachineStatus: internal.MachineStatus,
		TeeTimestamp:  internal.TeeTimestamp,
		InitialTeeId:  internal.InitialTeeId.Hex(),
		CodeHash:      hex.EncodeToString(internal.CodeHash[:]),
		Platform:      hex.EncodeToString(internal.Platform[:]),
		RewardEpochId: internal.RewardEpochId.String(),
	}
}

type AvailabilityCheckStatus uint8

// Match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L12
const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)

type TeeMachineStatus uint8

// Match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L13
const (
	INDETERMINATE TeeMachineStatus = 255 // TODO: check if this is ok
	ACTIVE        TeeMachineStatus = iota
	PAUSED
	PAUSED_FOR_UPGRADE
)

type ProxyInfoResponseBody struct {
	Data            ProxyInfoData   `json:"data"`        //TODO: ProxyInfoData TBD
	AttestationInfo AttestationInfo `json:"attestation"` //TODO: AttestationInfo TBD
}

type AttestationInfo struct {
	Platform    string `json:"platform"`
	Attestation string `json:"attestation"`
}

type ProxyInfoData struct {
	Challenge                string           `json:"challenge"`
	PublicKey                ecdsa.PublicKey  `json:"publicKey"`
	InitialTeeId             common.Address   `json:"initialTeeId"`
	Status                   TeeMachineStatus `json:"status"`
	InitialSigningPolicyId   *big.Int         `json:"initialSigningPolicyId"`
	InitialSigningPolicyHash common.Hash      `json:"initialSigningPolicyHash"`
	LastSigningPolicyId      *big.Int         `json:"lastSigningPolicyId"`
	LastSigningPolicyHash    common.Hash      `json:"lastSigningPolicyHash"`
	Nonce                    *big.Int         `json:"nonce"`
	PauseNonce               *big.Int         `json:"pauseNonce"`
	TeeTimestamp             uint64           `json:"teeTimestamp"`
}
