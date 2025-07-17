package middleware

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func APIKeyAuthMiddleware(apiKeys []string) func(ctx huma.Context, next func(huma.Context)) { // TODO return also body
	return func(ctx huma.Context, next func(huma.Context)) {
		apiKey := ctx.Header("X-API-KEY")
		if apiKey == "" {
			ctx.SetStatus(http.StatusUnauthorized)
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
			ctx.SetStatus(http.StatusUnauthorized)
			return
		}
		next(ctx)
	}
}
