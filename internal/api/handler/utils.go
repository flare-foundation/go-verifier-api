package handler

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func toIFTdcHubFtdcAttestationRequest(data types.FTDCRequestEncoded) (connector.IFtdcHubFtdcAttestationRequest, error) {
	encoded, err := hex.DecodeString(utils.RemoveHexPrefix(data.RequestBody))
	if err != nil {
		return connector.IFtdcHubFtdcAttestationRequest{}, err
	}
	return connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: common.HexToHash(data.FTDCHeader.AttestationType),
			SourceId:        common.HexToHash(data.FTDCHeader.SourceId),
			ThresholdBIPS:   data.FTDCHeader.ThresholdBIPS,
		},
		RequestBody: encoded,
	}, nil
}
