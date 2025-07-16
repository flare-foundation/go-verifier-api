package attestationtypes

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type AttestationRequestTeeAvailabilityCheck struct {
	Body struct {
		AttestationType string                           `json:"attestationType" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000" validate:"required,hash32"`
		SourceID        string                           `json:"sourceId" example:"0x7465650000000000000000000000000000000000000000000000000000000000" validate:"required,hash32"`
		RequestBody     ITeeAvailabilityCheckRequestBody `json:"requestBody" validate:"required"`
	}
}

type FullAttestationResponseTeeAvailabilityCheck struct {
	Body struct {
		AttestationStatus string `json:"attestationStatus"`
		Response          struct {
			AttestationType string                            `json:"attestationType"`
			SourceID        string                            `json:"sourceId"`
			RequestBody     ITeeAvailabilityCheckRequestBody  `json:"requestBody"`
			ResponseBody    ITeeAvailabilityCheckResponseBody `json:"responseBody"`
		} `json:"response"`
	}
}

// copied from connector.ITeeAvailabilityCheckRequestBody
type ITeeAvailabilityCheckRequestBody struct {
	TeeId     string `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge string `json:"challenge"  validate:"required,numeric" example:"12345678901234567890"`
}

// copied from connector.ITeeAvailabilityCheckResponseBody
type ITeeAvailabilityCheckResponseBody struct {
	Status        uint8
	MachineStatus uint8
	TeeTimestamp  uint64
	InitialTeeId  string
	CodeHash      string
	Platform      string
	RewardEpochId string
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
