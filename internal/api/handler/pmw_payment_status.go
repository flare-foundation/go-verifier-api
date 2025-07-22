package handler

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWPaymentStatusHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[types.PMWPaymentStatusRequestBody, types.PMWPaymentStatusResponseBody], sourceID string) {
	huma.Error501NotImplemented("PMW payment status not implemented yet")
}
