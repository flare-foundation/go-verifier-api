package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
)

func RegisterTeePoolingHandler(
	api huma.API,
	verifier *teeverifier.TeeVerifier,
) {
	registerOp(api,
		"get-polled-tees",
		http.MethodGet,
		"/poller/tees",
		[]string{"Poller"},
		func(ctx context.Context, request *struct{}) (*types.Response[types.TeeSamplesResponse], error) {
			samples := formatTeeSamples(verifier)
			return types.NewResponse(types.TeeSamplesResponse{Samples: samples}), nil
		})
}

func formatTeeSamples(teeVerifier *teeverifier.TeeVerifier) []verifiertypes.TeeSample {
	// Snapshot under lock — only copy sample slices, no formatting.
	teeVerifier.SamplesMu.RLock()
	snapshot := make(map[common.Address][]verifiertypes.TeeSampleValue, len(teeVerifier.TeeSamples))
	for teeID, values := range teeVerifier.TeeSamples {
		copied := make([]verifiertypes.TeeSampleValue, len(values))
		copy(copied, values)
		snapshot[teeID] = copied
	}
	teeVerifier.SamplesMu.RUnlock()

	// Format outside lock — Hex() and struct construction do not need synchronization.
	samples := make([]verifiertypes.TeeSample, 0, len(snapshot))
	for teeID, values := range snapshot {
		samples = append(samples, verifiertypes.TeeSample{
			TeeID:  teeID.Hex(),
			Values: values,
		})
	}
	return samples
}
