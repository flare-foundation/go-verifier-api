package types

// TeeSampleState classifies the outcome of a TEE attestation check used by
// the signing-policy and challenge-freshness paths.
type TeeSampleState uint8

const (
	TeeSampleValid         TeeSampleState = iota // successful and valid sample
	TeeSampleInvalid                             // attestation produced a definitive failure
	TeeSampleIndeterminate                       // could not determine (transient / verifier-side fault)
)

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
