package verifier

import (
	"context"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req attestationtypes.PMWMultisigAccountRequestBody) (attestationtypes.PMWMultisigAccountResponseBody, error) {
	// TODO
	return attestationtypes.PMWMultisigAccountResponseBody{}, nil
}
