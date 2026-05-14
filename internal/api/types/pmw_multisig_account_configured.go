package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
)

type PMWMultisigAccountConfiguredRequestBody struct {
	AccountAddress string          `json:"accountAddress" validate:"required" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	PublicKeys     []hexutil.Bytes `json:"publicKeys" validate:"required,min=1" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890,0x7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456"`
	Threshold      uint64          `json:"threshold" validate:"gte=1" example:"3"`
}

// MaxPublicKeys is the XRPL SignerList maximum:
// https://xrpl.org/docs/references/protocol/transactions/types/signerlistset
const MaxPublicKeys = 32

// ValidatePublicKeys enforces XRPL SignerList constraints on a decoded
// publicKeys slice. It is called by both the JSON request decoder (ToInternal)
// and by the verifier itself, so direct ABI requests cannot bypass the limit.
func ValidatePublicKeys(publicKeys [][]byte) error {
	if len(publicKeys) > MaxPublicKeys {
		return fmt.Errorf("too many public keys: %d (max %d)", len(publicKeys), MaxPublicKeys)
	}
	for i, pk := range publicKeys {
		if len(pk) == 0 {
			return fmt.Errorf("public key at index %d is empty", i)
		}
	}
	return nil
}

func (requestBody PMWMultisigAccountConfiguredRequestBody) ToInternal() (fdc2.IPMWMultisigAccountConfiguredRequestBody, error) {
	publicKeys := make([][]byte, len(requestBody.PublicKeys))
	for i, pk := range requestBody.PublicKeys {
		publicKeys[i] = pk
	}
	if err := ValidatePublicKeys(publicKeys); err != nil {
		return fdc2.IPMWMultisigAccountConfiguredRequestBody{}, err
	}

	return fdc2.IPMWMultisigAccountConfiguredRequestBody{
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

func (s PMWMultisigAccountConfiguredResponseBody) FromInternal(data fdc2.IPMWMultisigAccountConfiguredResponseBody) ResponseConvertible[fdc2.IPMWMultisigAccountConfiguredResponseBody] {
	return PMWMultisigAccountConfiguredResponseBody{
		PMWMultisigAccountStatus: data.Status,
		Sequence:                 data.Sequence,
	}
}

func (s PMWMultisigAccountConfiguredResponseBody) Log() {
	logger.Debugf("PMWMultisigAccountConfigured result: Status=%d, Sequence=%d",
		s.PMWMultisigAccountStatus, s.Sequence)
}

func LogPMWMultisigAccountConfiguredRequestBody(req fdc2.IPMWMultisigAccountConfiguredRequestBody) {
	logger.Debugf("PMWMultisigAccountConfigured request: AccountAddress=%s, Threshold=%d, PublicKeys=%d keys",
		req.AccountAddress, req.Threshold, len(req.PublicKeys))
}
