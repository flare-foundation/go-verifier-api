package attestationtypes

import (
	"encoding/json"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/ethereum/go-ethereum/common"
)

type TeeAvailabilityRequest = FTDCRequest[TeeAvailabilityRequestBody]

type TeeAvailabilityRequestBody struct {
	TeeId     string `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge string `json:"challenge" validate:"required,hash32" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

func (requestBody TeeAvailabilityRequestBody) ToInternal() (connector.ITeeAvailabilityCheckRequestBody, error) {
	return connector.ITeeAvailabilityCheckRequestBody{
		TeeId:     common.HexToAddress(requestBody.TeeId),
		Url:       requestBody.Url,
		Challenge: common.HexToHash(requestBody.Challenge),
	}, nil
}

type TeeAvailabilityResponseBody struct {
	Status                 json.Number                             `json:"status"`
	TeeTimestamp           json.Number                             `json:"teeTimestamp"`
	CodeHash               string                                  `json:"codeHash"`
	Platform               string                                  `json:"platform"`
	InitialSigningPolicyId json.Number                             `json:"initialSigningPolicyId"`
	LastSigningPolicyId    json.Number                             `json:"lastSigningPolicyId"`
	StateHash              connector.ITeeAvailabilityCheckTeeState `json:"state"`
}

func TeeToExternal(data connector.ITeeAvailabilityCheckResponseBody) TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:                 json.Number(fmt.Sprintf("%d", data.Status)),
		TeeTimestamp:           json.Number(fmt.Sprintf("%d", data.TeeTimestamp)),
		CodeHash:               common.BytesToHash(data.CodeHash[:]).Hex(),
		Platform:               common.BytesToHash(data.Platform[:]).Hex(),
		InitialSigningPolicyId: json.Number(fmt.Sprintf("%d", data.InitialSigningPolicyId)),
		LastSigningPolicyId:    json.Number(fmt.Sprintf("%d", data.LastSigningPolicyId)),
		StateHash: connector.ITeeAvailabilityCheckTeeState{
			SystemStateVersion: common.BytesToHash(data.State.SystemStateVersion[:]),
			SystemState:        data.State.SystemState,
			StateVersion:       common.BytesToHash(data.State.StateVersion[:]),
			State:              data.State.State,
		},
	}
}

type RawAndEncodedTeeAvailabilityResponseBody struct {
	ResponseData TeeAvailabilityResponseBody `json:"responseData"`
	ResponseBody string                      `json:"responseBody" example:"0x0000abcd..."`
}

type AvailabilityCheckStatus uint8

// Should match SC https://gitlab.com/flarenetwork/FSP/flare-smart-contracts-v2/-/blob/tee/contracts/userInterfaces/ftdc/ITeeAvailabilityCheck.sol?ref_type=heads#L12
const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)

type TeeSample struct {
	TeeID  string `json:"tee_id"`
	Values []bool `json:"values"`
}

type TeeSamplesResponse struct {
	Samples []TeeSample `json:"samples"`
}
