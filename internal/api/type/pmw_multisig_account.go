package attestationtypes

import (
	"encoding/hex"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

type PMWMultisigAccountRequest = FTDCRequest[PMWMultisigAccountRequestBody]

type PMWMultisigAccountRequestBody struct {
	WalletAddress string   `json:"walletAddress" validate:"required" example:"0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	PublicKeys    []string `json:"publicKeys" validate:"required,min=1" example:"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef,0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890,0x7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456"`
	Threshold     uint64   `json:"threshold" validate:"gte=1" example:"3"`
}

func (requestBody PMWMultisigAccountRequestBody) ToInternal() (connector.IPMWMultisigAccountConfiguredRequestBody, error) {
	var publicKeys [][]byte
	for _, pk := range requestBody.PublicKeys {
		b, err := hex.DecodeString(utils.RemoveHexPrefix(pk))
		if err != nil {
			return connector.IPMWMultisigAccountConfiguredRequestBody{}, fmt.Errorf("invalid public key: %s, err: %w", pk, err)
		}
		publicKeys = append(publicKeys, b)
	}

	return connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: requestBody.WalletAddress,
		PublicKeys:    publicKeys,
		Threshold:     requestBody.Threshold,
	}, nil
}

type PMWMultisigAccountResponseBody struct {
	PMWMultisigAccountStatus uint8  `json:"status"`
	Sequence                 uint64 `json:"sequence"`
}

type PMWMultisigAccountStatus int

const (
	PMWMultisigAccountStatusOK PMWMultisigAccountStatus = iota
	PMWMultisigAccountStatusERROR
)

func MultiSigToExternal(data connector.IPMWMultisigAccountConfiguredResponseBody) PMWMultisigAccountResponseBody {
	return PMWMultisigAccountResponseBody{
		PMWMultisigAccountStatus: data.Status,
		Sequence:                 data.Sequence,
	}
}

type RawAndEncodedPMWMultisigAccountResponseBody = RawAndEncodedFTDCResponse[PMWMultisigAccountResponseBody]
