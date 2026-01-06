package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
)

func RegisterHealthHandler(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/api/health",
		Tags:        []string{"Health"},
		Security:    []map[string][]string{},
	},
		func(ctx context.Context, req *struct{}) (*types.Response[types.HealthCheckResponse], error) {
			resp := types.HealthCheckResponse{
				Healthy: true,
			}
			return types.NewResponse(resp), nil
		},
	)
}
