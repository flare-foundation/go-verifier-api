package types

import (
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TeeAvailabilityCheckRequestBody struct {
	TeeID         common.Address `json:"teeId" validate:"required" example:"0x000000000000000000000000000000000000dEaD"`
	TeeProxyID    common.Address `json:"teeProxyId" validate:"required" example:"0x000000000000000000000000000000000000dEaD"`
	URL           string         `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge     common.Hash    `json:"challenge" validate:"required" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
	InstructionID common.Hash    `json:"instructionId" validate:"required" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

func (requestBody TeeAvailabilityCheckRequestBody) ToInternal() (connector.ITeeAvailabilityCheckRequestBody, error) {
	return connector.ITeeAvailabilityCheckRequestBody{
		TeeId:         requestBody.TeeID,
		TeeProxyId:    requestBody.TeeProxyID,
		Url:           requestBody.URL,
		Challenge:     requestBody.Challenge,
		InstructionId: requestBody.InstructionID,
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

func (t TeeAvailabilityCheckResponseBody) FromInternal(data connector.ITeeAvailabilityCheckResponseBody) ResponseConvertible[connector.ITeeAvailabilityCheckResponseBody] {
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

func (t TeeAvailabilityCheckResponseBody) Log() {
	logger.Debugf("TEEAvailabilityCheck result: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyID:%d, LastSigningPolicyID=%d, State=%v",
		t.Status,
		t.TeeTimestamp,
		t.CodeHash,
		convert.CommonHashToString(t.Platform),
		t.InitialSigningPolicyID,
		t.LastSigningPolicyID,
		t.State)
}

func LogTeeAvailabilityCheckRequestBody(req connector.ITeeAvailabilityCheckRequestBody) {
	logger.Debugf("TeeAvailabilityCheck request: TeeID=%s, TeeProxyID=%s, URL=%s, Challenge=%x, InstructionID=%x",
		req.TeeId, req.TeeProxyId, req.Url, req.Challenge, req.InstructionId)
}

type TeeSamplesResponse struct {
	Samples []verifiertypes.TeeSample `json:"samples"`
	Total   int                       `json:"total"`
}
