package types

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type RequestBody struct {
	TeeId     common.Address `json:"teeId"`
	URL       string         `json:"url"`
	Challenge *big.Int       `json:"challenge"`
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
	UNDETERMINED TeeMachineStatus = 255 // TODO
	ACTIVE       TeeMachineStatus = iota
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
	TeeGovernanceHash        common.Hash      `json:"teeGovernanceHash"`
	TeeTimestamp             uint64           `json:"teeTimestamp"`
}
