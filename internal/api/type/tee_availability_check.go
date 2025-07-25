package attestationtypes

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/tee"
)

type TeeAvailabilityHeader struct {
	AttestationType    string   `json:"attestationType" validate:"required,hash32" example:"0x546565417661696c6162696c697479436865636b000000000000000000000000"`
	SourceId           string   `json:"sourceId" validate:"required,hash32" example:"0x7465650000000000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS      uint16   `json:"thresholdBIPS" example:"0"`
	Cosigners          []string `json:"cosigners" example:"[]"`
	CosignersThreshold uint64   `json:"cosignersThreshold" example:"0"`
}

type TeeAvailabilityEncodedRequest struct {
	FTDCHeader  TeeAvailabilityHeader `json:"header"`
	RequestBody string                `json:"requestBody" example:"0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"`
}
type TeeAvailabilityRequest struct {
	FTDCHeader  TeeAvailabilityHeader      `json:"header"`
	RequestData TeeAvailabilityRequestBody `json:"requestData"`
}

type TeeAvailabilityRequestBody struct {
	TeeId     string `json:"teeId" validate:"required,eth_addr" example:"0x000000000000000000000000000000000000dEaD"`
	Url       string `json:"url" validate:"required,url" example:"https://supertee.proxy"`
	Challenge string `json:"challenge" validate:"required,hash32" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"`
}

type TeeAvailabilityRequestData struct {
	TeeId     common.Address
	Url       string
	Challenge common.Hash
}

func (requestBody TeeAvailabilityRequestBody) ToInternal() (TeeAvailabilityRequestData, error) {
	return TeeAvailabilityRequestData{
		TeeId:     common.HexToAddress(requestBody.TeeId),
		Url:       requestBody.Url,
		Challenge: common.HexToHash(requestBody.Challenge),
	}, nil
}

type TeeAvailabilityResponseBody struct {
	Status                 json.Number `json:"status"`
	TeeTimestamp           json.Number `json:"teeTimestamp"`
	CodeHash               string      `json:"codeHash"`
	Platform               string      `json:"platform"`
	InitialSigningPolicyId json.Number `json:"initialSigningPolicyId"`
	LastSigningPolicyId    json.Number `json:"lastSigningPolicyId"`
	StateHash              string      `json:"stateHash"`
}
type TeeAvailabilityResponseData struct {
	Status                 uint8       `json:"status"`
	TeeTimestamp           uint64      `json:"teeTimestamp"`
	CodeHash               common.Hash `json:"codeHash"`
	Platform               common.Hash `json:"platform"`
	InitialSigningPolicyId uint32      `json:"initialSigningPolicyId"`
	LastSigningPolicyId    uint32      `json:"lastSigningPolicyId"`
	StateHash              common.Hash `json:"stateHash"`
}

func (data TeeAvailabilityResponseData) ToExternal() TeeAvailabilityResponseBody {
	return TeeAvailabilityResponseBody{
		Status:                 json.Number(fmt.Sprintf("%d", data.Status)),
		TeeTimestamp:           json.Number(fmt.Sprintf("%d", data.TeeTimestamp)),
		CodeHash:               data.CodeHash.Hex(),
		Platform:               data.Platform.Hex(),
		InitialSigningPolicyId: json.Number(fmt.Sprintf("%d", data.InitialSigningPolicyId)),
		LastSigningPolicyId:    json.Number(fmt.Sprintf("%d", data.LastSigningPolicyId)),
		StateHash:              data.StateHash.Hex(),
	}
}

type RawAndEncodedResponseBody struct {
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

// copied from https://gitlab.com/flarenetwork/tee/tee-node/-/blob/brezTilna/pkg/types/tee.go?ref_type=heads#L16
type TeeInfoResponse struct {
	TeeInfo     tee.TeeStructsAttestation
	State       []byte
	Version     string
	Attestation hexutil.Bytes
	//TODO if platform will be added in tee-node, it also needs to be added here
}

// copied from here: https://gitlab.com/flarenetwork/tee/tee-node/-/blob/main/pkg/types/actions.go?ref_type=heads#L40
type ActionType string

const (
	Instruction ActionType = "instruction"
	Direct      ActionType = "direct"
)

type SubmissionTag string

const (
	Threshold SubmissionTag = "threshold"
	End       SubmissionTag = "end"
	Submit    SubmissionTag = "submit"
)

type Action struct {
	Data                       ActionData      `json:"data"`
	AdditionalVariableMessages []hexutil.Bytes `json:"additionalVariableMessages"`
	Timestamps                 []uint64        `json:"timestamps"`
	AdditionalActionData       hexutil.Bytes   `json:"additionalActionData"`
	Signatures                 []hexutil.Bytes `json:"signatures"`
}

type ActionData struct {
	ID            common.Hash   `json:"id"`
	Type          ActionType    `json:"type"`
	SubmissionTag SubmissionTag `json:"submissionTag"`
	Message       hexutil.Bytes `json:"message"`
}

type ActionResponse struct {
	Result    ActionResult  `json:"result"`
	Signature hexutil.Bytes `json:"signature"`
}

type ActionResult struct {
	ID            common.Hash   `json:"id"`
	SubmissionTag SubmissionTag `json:"submissionTag"`
	Status        bool          `json:"status"`
	Log           string        `json:"log"`

	OPType                 common.Hash   `json:"opType"`
	OPCommand              common.Hash   `json:"opCommand"`
	AdditionalResultStatus hexutil.Bytes `json:"additionalResultStatus"`

	Version string        `json:"version"`
	Data    hexutil.Bytes `json:"message"`
}
