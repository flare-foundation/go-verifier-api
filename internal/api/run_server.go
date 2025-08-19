package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/flare-foundation/go-verifier-api/internal/api/middleware"
	"github.com/rs/cors"
	"github.com/unrolled/secure"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	apidocs "github.com/flare-foundation/go-verifier-api/internal/api-docs"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func RunServer() {
	_ = godotenv.Load()

	verifierTypeStr := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	sourceIdStr := os.Getenv("SOURCE_ID")
	if verifierTypeStr == "" || port == "" || sourceIdStr == "" {
		logger.Fatal("VERIFIER_TYPE, PORT and SOURCE_ID must be set")
	}
	attestationType, err := parseAttestationType(verifierTypeStr)
	if err != nil {
		logger.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}
	sourceId, err := parseSourceId(sourceIdStr)
	if err != nil {
		logger.Fatalf("Invalid SOURCE_ID in .env: %v", err)
	}
	apiKeys, err := getAPIKeys()
	if err != nil {
		logger.Fatalf("%v", err)
	}

	router := chi.NewRouter()
	config := huma.DefaultConfig("FTDC Verifier API", "1.0")
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
	api.UseMiddleware(middleware.APIKeyAuthMiddleware(api, apiKeys))

	// swagger setup
	router.Get("/api-doc", apidocs.SwaggerIndexHandler)
	router.Get("/api-doc/*", apidocs.SwaggerFileHandler)

	err = LoadModule(api, sourceId, attestationType)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:               true,
		STSSeconds:                15552000,
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
		IsDevelopment:             os.Getenv("ENV") == "development", // TODO can this be handled in a better way?
	})
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})
	routerWithSecurity := secureMiddleware.Handler(router)
	routerWithCORS := corsHandler.Handler(routerWithSecurity)

	fmt.Printf("Starting server on: %s...\n", port)
	logger.Fatal(http.ListenAndServe(": "+port, routerWithCORS))
}

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
}

var sourceIds = []config.SourceName{
	config.SourceTEE,
	config.SourceXRP,
}

func parseAttestationType(value string) (connector.AttestationType, error) {
	for _, at := range attestationTypes {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

func parseSourceId(value string) (config.SourceName, error) {
	for _, at := range sourceIds {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

func getAPIKeys() ([]string, error) {
	raw := os.Getenv("API_KEYS")
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("API_KEYS must be set")
	}
	var apiKeys []string
	for _, key := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			apiKeys = append(apiKeys, trimmed)
		}
	}
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("API_KEYS contains only empty values")
	}
	return apiKeys, nil
}
