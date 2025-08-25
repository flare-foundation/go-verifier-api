package middleware

import (
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
)

func APIKeyAuthMiddleware(api huma.API, apiKeys []string) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// skip /health endpoint
		if ctx.URL().Path == "/api/health" {
			next(ctx)
			return
		}
		apiKey := ctx.Header("X-API-KEY")
		if apiKey == "" {
			err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			if err != nil {
				logger.Error(err)
			}
			return
		}
		found := false
		for _, key := range apiKeys {
			if apiKey == key {
				found = true
				break
			}
		}
		if !found {
			err := huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			if err != nil {
				logger.Error(err)
			}
			return
		}
		next(ctx)
	}
}
