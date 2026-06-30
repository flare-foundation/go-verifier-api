package types

import (
	"errors"
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

// ValidateMultisigRequest enforces the full multisig request shape: at least one
// public key (in addition to the per-key and cap checks of ValidatePublicKeys) and
// a non-zero threshold — a zero signer quorum is never a valid multisig. These
// mirror the JSON `min=1`/`gte=1` validation tags so direct ABI requests, which
// skip JSON validation, cannot bypass them.
func ValidateMultisigRequest(publicKeys [][]byte, threshold uint64) error {
	if len(publicKeys) == 0 {
		return errors.New("publicKeys must not be empty")
	}
	if err := ValidatePublicKeys(publicKeys); err != nil {
		return err
	}
	if threshold == 0 {
		return errors.New("threshold must be greater than zero")
	}
	return nil
}

func (requestBody PMWMultisigAccountConfiguredRequestBody) ToInternal() (fdc2.IPMWMultisigAccountConfiguredRequestBody, error) {
	publicKeys := make([][]byte, len(requestBody.PublicKeys))
	for i, pk := range requestBody.PublicKeys {
		publicKeys[i] = pk
	}
	// Same validator the verifier uses, so the JSON and ABI paths share one source
	// of truth (the JSON `min=1`/`gte=1` tags catch these earlier, but keeping the
	// check here avoids split-brain validation if a tag is changed or removed).
	if err := ValidateMultisigRequest(publicKeys, requestBody.Threshold); err != nil {
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
