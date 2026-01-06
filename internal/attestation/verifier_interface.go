package attestation

import (
	"context"
)

type Verifier[Req any, Res any] interface {
	Verify(ctx context.Context, req Req) (Res, error)
}
