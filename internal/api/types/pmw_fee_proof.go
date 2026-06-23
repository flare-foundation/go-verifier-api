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
	FirstPaymentId uint64      `json:"firstPaymentId" validate:"required" example:"1"`
	BatchCount     uint64      `json:"batchCount" validate:"required" example:"10"`
	UntilTimestamp uint64      `json:"untilTimestamp" validate:"required" example:"1710000000"`
}

func (requestBody PMWFeeProofRequestBody) ToInternal() (fdc2.IPMWFeeProofRequestBody, error) {
	return fdc2.IPMWFeeProofRequestBody{
		OpType:         requestBody.OpType,
		SenderAddress:  requestBody.SenderAddress,
		FirstPaymentId: requestBody.FirstPaymentId,
		BatchCount:     requestBody.BatchCount,
		UntilTimestamp: requestBody.UntilTimestamp,
	}, nil
}

type PMWFeeProofResponseBody struct {
	LastPaymentId uint64      `json:"lastPaymentId"`
	ActualFee     hexutil.Big `json:"actualFee"`
	EstimatedFee  hexutil.Big `json:"estimatedFee"`
}

func (s PMWFeeProofResponseBody) FromInternal(data fdc2.IPMWFeeProofResponseBody) ResponseConvertible[fdc2.IPMWFeeProofResponseBody] {
	return PMWFeeProofResponseBody{
		LastPaymentId: data.LastPaymentId,
		ActualFee:     hexutil.Big(*data.ActualFee),
		EstimatedFee:  hexutil.Big(*data.EstimatedFee),
	}
}

func (s PMWFeeProofResponseBody) Log() {
	logger.Debugf("PMWFeeProof result: LastPaymentId=%d, ActualFee=%v, EstimatedFee=%v",
		s.LastPaymentId,
		s.ActualFee,
		s.EstimatedFee,
	)
}

func LogPMWFeeProofRequestBody(req fdc2.IPMWFeeProofRequestBody) {
	logger.Debugf("PMWFeeProof request: OpType=%s, SenderAddress=%s, FirstPaymentId=%d, BatchCount=%d, UntilTimestamp=%d",
		convert.CommonHashToString(req.OpType), req.SenderAddress, req.FirstPaymentId, req.BatchCount, req.UntilTimestamp)
}
