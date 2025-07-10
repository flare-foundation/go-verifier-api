package verifierinterface

import (
	"context"
)

type VerifierInterface[Req any, Res any] interface {
	Verify(ctx context.Context, input Req) (Res, error)
}
