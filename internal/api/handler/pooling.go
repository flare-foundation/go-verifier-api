package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
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
	teeVerifier.SamplesMu.RLock()
	defer teeVerifier.SamplesMu.RUnlock()
	samples := make([]verifiertypes.TeeSample, 0, len(teeVerifier.TeeSamples))
	for teeID, values := range teeVerifier.TeeSamples {
		sampleValues := make([]verifiertypes.TeeSampleValue, 0, len(values))
		sampleValues = append(sampleValues, values...)

		samples = append(samples, verifiertypes.TeeSample{
			TeeID:  teeID.Hex(),
			Values: sampleValues,
		})
	}

	return samples
}
