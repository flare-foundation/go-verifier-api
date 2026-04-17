package handler

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
)

type pollerTeesRequest struct {
	Offset int `query:"offset" minimum:"0" default:"0" doc:"Number of entries to skip."`
	Limit  int `query:"limit" minimum:"1" maximum:"500" default:"100" doc:"Max entries to return."`
}

func RegisterTeePoolingHandler(
	api huma.API,
	verifier *teeverifier.TeeVerifier,
) {
	registerOp(api,
		"get-polled-tees",
		http.MethodGet,
		"/poller/tees",
		[]string{"Poller"},
		func(ctx context.Context, request *pollerTeesRequest) (*types.Response[types.TeeSamplesResponse], error) {
			all := loadSnapshot(&verifier.PollerSnapshot)
			total := len(all)
			offset := request.Offset
			limit := request.Limit
			if offset > total {
				offset = total
			}
			end := min(offset+limit, total)
			return types.NewResponse(types.TeeSamplesResponse{
				Samples: all[offset:end],
				Total:   total,
			}), nil
		})
}

func loadSnapshot(snap *atomic.Value) []verifiertypes.TeeSample {
	v, _ := snap.Load().([]verifiertypes.TeeSample)
	return v
}
