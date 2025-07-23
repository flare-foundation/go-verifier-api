package middleware

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func APIKeyAuthMiddleware(api huma.API, apiKeys []string) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		apiKey := ctx.Header("X-API-KEY")
		if apiKey == "" {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
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
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}
		next(ctx)
	}
}
