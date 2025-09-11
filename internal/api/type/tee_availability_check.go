package attestationtypes

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TeeAvailabilityCheckRequestBody struct {
	TeeID     common.Address `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	URL       string         `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge common.Hash    `json:"challenge" validate:"required,hash32" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

func (requestBody TeeAvailabilityCheckRequestBody) ToInternal() (connector.ITeeAvailabilityCheckRequestBody, error) {
	return connector.ITeeAvailabilityCheckRequestBody{
		TeeId:     requestBody.TeeID,
		Url:       requestBody.URL,
		Challenge: requestBody.Challenge,
	}, nil
}

type TeeAvailabilityCheckResponseBody struct {
	Status                 uint8                        `json:"status"`
	TeeTimestamp           uint64                       `json:"teeTimestamp"`
	CodeHash               common.Hash                  `json:"codeHash"`
	Platform               common.Hash                  `json:"platform"`
	InitialSigningPolicyID uint32                       `json:"initialSigningPolicyId"`
	LastSigningPolicyID    uint32                       `json:"lastSigningPolicyId"`
	State                  TeeAvailabilityCheckTeeState `json:"state"`
}

type TeeAvailabilityCheckTeeState struct {
	SystemState        hexutil.Bytes `json:"systemState"`
	SystemStateVersion common.Hash   `json:"systemStateVersion"`
	State              hexutil.Bytes `json:"state"`
	StateVersion       common.Hash   `json:"stateVersion"`
}

func TeeAvailabilityCheckToExternal(data connector.ITeeAvailabilityCheckResponseBody) TeeAvailabilityCheckResponseBody {
	return TeeAvailabilityCheckResponseBody{
		Status:                 data.Status,
		TeeTimestamp:           data.TeeTimestamp,
		CodeHash:               data.CodeHash,
		Platform:               data.Platform,
		InitialSigningPolicyID: data.InitialSigningPolicyId,
		LastSigningPolicyID:    data.LastSigningPolicyId,
		State: TeeAvailabilityCheckTeeState{
			SystemStateVersion: data.State.SystemStateVersion,
			SystemState:        data.State.SystemState,
			StateVersion:       data.State.StateVersion,
			State:              data.State.State,
		},
	}
}

type TeeSamplesResponse struct {
	Samples []teetype.TeeSample `json:"samples"`
}
