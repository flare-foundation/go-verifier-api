package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
)

type PMWFeeProofRequestBody struct {
	OpType         common.Hash `json:"opType" validate:"required" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	SenderAddress  string      `json:"senderAddress" validate:"required" example:"abcdef"`
	FromNonce      uint64      `json:"fromNonce" validate:"required" example:"1"`
	ToNonce        uint64      `json:"toNonce" validate:"required" example:"10"`
	UntilTimestamp uint64      `json:"untilTimestamp" validate:"required" example:"1710000000"`
}

func (requestBody PMWFeeProofRequestBody) ToInternal() (fdc2.IPMWFeeProofRequestBody, error) {
	return fdc2.IPMWFeeProofRequestBody{
		OpType:         requestBody.OpType,
		SenderAddress:  requestBody.SenderAddress,
		FromNonce:      requestBody.FromNonce,
		ToNonce:        requestBody.ToNonce,
		UntilTimestamp: requestBody.UntilTimestamp,
	}, nil
}

type PMWFeeProofResponseBody struct {
	ActualFee    hexutil.Big `json:"actualFee"`
	EstimatedFee hexutil.Big `json:"estimatedFee"`
}

func (s PMWFeeProofResponseBody) FromInternal(data fdc2.IPMWFeeProofResponseBody) ResponseConvertible[fdc2.IPMWFeeProofResponseBody] {
	return PMWFeeProofResponseBody{
		ActualFee:    hexutil.Big(*data.ActualFee),
		EstimatedFee: hexutil.Big(*data.EstimatedFee),
	}
}

func (s PMWFeeProofResponseBody) Log() {
	logger.Debugf("PMWFeeProof result: ActualFee=%v, EstimatedFee=%v",
		s.ActualFee,
		s.EstimatedFee,
	)
}

func LogPMWFeeProofRequestBody(req fdc2.IPMWFeeProofRequestBody) {
	logger.Debugf("PMWFeeProof request: OpType=%s, SenderAddress=%s, FromNonce=%d, ToNonce=%d, UntilTimestamp=%d",
		convert.CommonHashToString(req.OpType), req.SenderAddress, req.FromNonce, req.ToNonce, req.UntilTimestamp)
}
