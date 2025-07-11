package verifierinterface

import (
	"context"

	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
)

type VerifierInterface[Req any, Res any] interface {
	Verify(ctx context.Context, req Req) (attestationtypes.AttestationResponseStatus, Res, error)
}
