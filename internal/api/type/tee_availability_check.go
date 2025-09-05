package attestationtypes

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TeeAvailabilityRequestBody struct {
	TeeId     common.Address `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string         `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge common.Hash    `json:"challenge" validate:"required,hash32" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

func (requestBody TeeAvailabilityRequestBody) ToInternal() (connector.ITeeAvailabilityCheckRequestBody, error) {
	return connector.ITeeAvailabilityCheckRequestBody{
		TeeId:     requestBody.TeeId,
		Url:       requestBody.Url,
		Challenge: requestBody.Challenge,
	}, nil
}

type TeeAvailabilityResponseBody struct {
	Status                 uint8                        `json:"status"`
	TeeTimestamp           uint64                       `json:"teeTimestamp"`
	CodeHash               common.Hash                  `json:"codeHash"`
	Platform               common.Hash                  `json:"platform"`
	InitialSigningPolicyId uint32                       `json:"initialSigningPolicyId"`
	LastSigningPolicyId    uint32                       `json:"lastSigningPolicyId"`
	StateHash              TeeAvailabilityCheckTeeState `json:"state"`
}

type TeeAvailabilityCheckTeeState struct {
	SystemState        hexutil.Bytes `json:"systemState"`
	SystemStateVersion common.Hash   `json:"systemStateVersion"`
	State              hexutil.Bytes `json:"state"`
	StateVersion       common.Hash   `json:"stateVersion"`
}

func TeeToExternal(data connector.ITeeAvailabilityCheckResponseBody) TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:                 data.Status,
		TeeTimestamp:           data.TeeTimestamp,
		CodeHash:               data.CodeHash,
		Platform:               data.Platform,
		InitialSigningPolicyId: data.InitialSigningPolicyId,
		LastSigningPolicyId:    data.LastSigningPolicyId,
		StateHash: TeeAvailabilityCheckTeeState{
			SystemStateVersion: data.State.SystemStateVersion,
			SystemState:        data.State.SystemState,
			StateVersion:       data.State.StateVersion,
			State:              data.State.State,
		},
	}
}

type TeeSamplesResponse struct {
	Samples []teetypes.TeeSample `json:"samples"`
}
