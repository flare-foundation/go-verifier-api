package verifier

import (
	"context"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/config"
)

type XRPVerifier struct {
	config *pmwmultisigaccountconfig.PMWMultisigAccountConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWMultisigAccountConfiguredRequestBody) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	// TODO
	return connector.IPMWMultisigAccountConfiguredResponseBody{}, nil
}
