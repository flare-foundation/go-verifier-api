package teetypes

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
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

type StatusInfo struct {
	CodeHash common.Hash
	Platform common.Hash
	Status   AvailabilityCheckStatus
}
