package e2e

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	internalApi "github.com/flare-foundation/go-verifier-api/internal/api"
	apidocs "github.com/flare-foundation/go-verifier-api/internal/api-docs"
	"github.com/flare-foundation/go-verifier-api/internal/api/middleware"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	"github.com/unrolled/secure"
)

func RunServer(envConfig config.EnvConfig) {
	router := chi.NewRouter()
	config := huma.DefaultConfig("FTDC Verifier API", "1.0")
	config.Info.Description = "The FTDC Verifier API endpoints"

	config.DocsPath = ""
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"ApiKeyAuth": {
			Type: "apiKey",
			In:   "header",
			Name: "X-API-KEY",
		},
	}
	config.Security = []map[string][]string{
		{"ApiKeyAuth": {}},
	}
	api := humachi.New(router, config)
	api.UseMiddleware(middleware.APIKeyAuthMiddleware(api, envConfig.ApiKeys))

	// swagger setup
	router.Get("/api-doc", apidocs.SwaggerIndexHandler)
	router.Get("/api-doc/*", apidocs.SwaggerFileHandler)

	err := internalApi.LoadModule(api, envConfig)
	if err != nil {
		logger.Fatalf("%v", err)
	}
	const (
		SecondsPerDay        = 24 * 60 * 60
		STSDurationInSeconds = 180 * SecondsPerDay
	)
	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:               true,
		STSSeconds:                STSDurationInSeconds,
		STSIncludeSubdomains:      true,
		STSPreload:                true,
		ForceSTSHeader:            true,
		FrameDeny:                 true,
		ContentTypeNosniff:        true,
		ReferrerPolicy:            "no-referrer",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		XDNSPrefetchControl:       "off",
		IsDevelopment:             true,
	})
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})
	routerWithSecurity := secureMiddleware.Handler(router)
	routerWithCORS := corsHandler.Handler(routerWithSecurity)

	logger.Infof("Starting %s verifier server with type %s on: %s ...\n", envConfig.SourceID, envConfig.AttestationType, envConfig.Port)
	logger.Fatal(http.ListenAndServe(":"+envConfig.Port, routerWithCORS))
}
