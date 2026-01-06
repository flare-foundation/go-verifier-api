package types

import (
	"time"
)

type TeeSampleValue struct {
	Timestamp time.Time      `json:"timestamp"`
	State     TeeSampleState `json:"state"`
}

type TeeSampleState uint8

const (
	TeeSampleValid TeeSampleState = iota // successful and valid sample
	TeeSampleInvalid
	TeeSampleIndeterminate
)

type TeeSample struct {
	TeeID  string           `json:"tee_id"`
	Values []TeeSampleValue `json:"values"`
}

func (s TeeSampleState) String() string {
	switch s {
	case TeeSampleValid:
		return "VALID"
	case TeeSampleInvalid:
		return "INVALID"
	case TeeSampleIndeterminate:
		return "INDETERMINATE"
	default:
		return "UNKNOWN"
	}
}

func (s TeeSampleState) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}
