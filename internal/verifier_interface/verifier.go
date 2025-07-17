package verifierinterface

import (
	"context"
)

type VerifierInterface[Req any, Res any] interface {
	Verify(ctx context.Context, req Req) (Res, error)
}
