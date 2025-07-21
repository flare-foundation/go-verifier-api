package crypto

import (
	types "gitlab.com/urskak/verifier-api/internal/api/type"
)

func AbiEncodeRequestData(data types.TeeAvailabilityRequestData) ([]byte, error) {
	arguments, err := AbiArgumentsForRequestData()
	if err != nil {
		return nil, err
	}
	return arguments.Pack(data.TeeId, data.Url, data.Challenge)
}

func AbiEncodeResponseData(data types.TeeAvailabilityResponseData) ([]byte, error) {
	arguments, err := AbiArgumentsForResponseData()
	if err != nil {
		return nil, err
	}
	return arguments.Pack(data.Status, data.TeeTimestamp, data.CodeHash, data.Platform, data.LastSigningPolicyId, data.InitialSigningPolicyId, data.StateHash)
}
