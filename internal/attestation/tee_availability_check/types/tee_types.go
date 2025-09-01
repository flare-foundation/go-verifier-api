package teetypes

import (
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang-jwt/jwt/v4"
)

type TeeSample struct {
	TeeID  string           `json:"tee_id"`
	Values []TeeSampleValue `json:"values"`
}

type TeeSampleValue struct {
	Timestamp time.Time            `json:"timestamp"`
	State     TeePollerSampleState `json:"state"`
}

type AvailabilityCheckStatus uint8

// Should match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L12
const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)

type TeePollerSampleState uint8

const (
	TeePollerSampleValid TeePollerSampleState = iota // successful and valid sample
	TeePollerSampleInvalid
	TeePollerSampleIndeterminate
)

type TeePollerSample struct {
	Timestamp time.Time
	State     TeePollerSampleState
}

func (s TeePollerSampleState) String() string {
	switch s {
	case TeePollerSampleValid:
		return "VALID"
	case TeePollerSampleInvalid:
		return "INVALID"
	case TeePollerSampleIndeterminate:
		return "INDETERMINATE"
	default:
		return "UNKNOWN"
	}
}

func (s TeePollerSampleState) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

type EatNonce []string

func (e *EatNonce) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*e = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*e = []string{s}
		return nil
	}
	*e = []string{}
	return nil
}

type GoogleTeeClaims struct {
	HWModel     string     `json:"hwmodel"`
	SWName      string     `json:"swname"`
	SecBoot     bool       `json:"secboot"`
	EATNonce    EatNonce   `json:"eat_nonce"`
	SubMods     SubModules `json:"submods"`
	DebugStatus string     `json:"dbgstat"`
	jwt.StandardClaims
}

type SubModules struct {
	ConfidentialSpace ConfidentialSpaceInfo `json:"confidential_space"`
	Container         Container             `json:"container"`
}

type ConfidentialSpaceInfo struct {
	SupportAttributes []string `json:"support_attributes"`
}

type Container struct {
	ImageDigest string `json:"image_digest"`
	ImageId     string `json:"image_id"`
}

type StatusInfo struct {
	CodeHash common.Hash
	Platform common.Hash
	Status   AvailabilityCheckStatus
}
