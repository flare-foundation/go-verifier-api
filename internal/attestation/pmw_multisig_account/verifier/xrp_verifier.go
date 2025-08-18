package verifier

import (
	"context"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req attestationtypes.PMWMultisigAccountRequestData) (attestationtypes.PMWMultisigAccountResponseData, error) {
	// TODO
	return attestationtypes.PMWMultisigAccountResponseData{}, nil
}
