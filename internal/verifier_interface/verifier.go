package verifierinterface

import (
	"context"

	attestationtypes "gitlab.com/urskak/verifier-api/internal/common"
)

type VerifierInterface[Req any, Res any] interface {
	Verify(ctx context.Context, input Req) (attestationtypes.AttestationResponseStatus, Res, error)
}
