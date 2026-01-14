package types

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type PMWMultisigAccountConfiguredRequestBody struct {
	AccountAddress string          `json:"accountAddress" validate:"required" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	PublicKeys     []hexutil.Bytes `json:"publicKeys" validate:"required,min=1" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890,0x7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456"`
	Threshold      uint64          `json:"threshold" validate:"gte=1" example:"3"`
}

func (requestBody PMWMultisigAccountConfiguredRequestBody) ToInternal() (connector.IPMWMultisigAccountConfiguredRequestBody, error) {
	publicKeys := make([][]byte, len(requestBody.PublicKeys))
	for i, pk := range requestBody.PublicKeys {
		if len(pk) == 0 {
			return connector.IPMWMultisigAccountConfiguredRequestBody{}, fmt.Errorf("public key at index %d is empty", i)
		}
		publicKeys[i] = pk
	}

	return connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: requestBody.AccountAddress,
		PublicKeys:     publicKeys,
		Threshold:      requestBody.Threshold,
	}, nil
}

type PMWMultisigAccountConfiguredResponseBody struct {
	PMWMultisigAccountStatus uint8  `json:"status"`
	Sequence                 uint64 `json:"sequence"`
}

type PMWMultisigAccountConfiguredStatus int

const (
	PMWMultisigAccountStatusOK PMWMultisigAccountConfiguredStatus = iota
	PMWMultisigAccountStatusERROR
)

func (s PMWMultisigAccountConfiguredResponseBody) FromInternal(data connector.IPMWMultisigAccountConfiguredResponseBody) ResponseConvertible[connector.IPMWMultisigAccountConfiguredResponseBody] {
	return PMWMultisigAccountConfiguredResponseBody{
		PMWMultisigAccountStatus: data.Status,
		Sequence:                 data.Sequence,
	}
}

func (s PMWMultisigAccountConfiguredResponseBody) Log() {
	logger.Debugf("PMWMultisigAccountConfigured result: Status=%d, Sequence=%d",
		s.PMWMultisigAccountStatus, s.Sequence)
}

func LogPMWMultisigAccountConfiguredRequestBody(req connector.IPMWMultisigAccountConfiguredRequestBody) {
	logger.Debugf("PMWMultisigAccountConfigured request: AccountAddress=%s, Threshold=%d, PublicKeys=%s",
		req.AccountAddress, req.Threshold, formatPublicKeys(req.PublicKeys))
}

func formatPublicKeys(pks [][]byte) string {
	out := make([]string, len(pks))
	for i, pk := range pks {
		out[i] = fmt.Sprintf("%x", pk)
	}
	return strings.Join(out, ",")
}
