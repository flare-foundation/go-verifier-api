package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/middleware"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
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

	router.Get("/api-doc", swagger)

	err = LoadModule(api, sourceId, attestationType)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	fmt.Printf("Starting server on :%s...\n", port)
	logger.Fatal(http.ListenAndServe(":"+port, router))
}

func parseAttestationType(value string) (connector.AttestationType, error) {
	for _, at := range attestationTypes {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
}

func parseSourceId(value string) (config.SourceName, error) {
	for _, at := range sourceIds {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

var sourceIds = []config.SourceName{
	config.SourceTEE,
	config.SourceXRP,
}

func getAPIKeys() ([]string, error) {
	raw := os.Getenv("API_KEYS")
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("API_KEYS must be set")
	}

	// Split, trim, and filter
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

func swagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
			<html lang="en">
			<head>
			<meta charset="utf-8" />
			<meta name="viewport" content="width=device-width, initial-scale=1" />
			<meta name="description" content="SwaggerUI" />
			<title>SwaggerUI</title>
			<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
			</head>
			<body>
			<div id="swagger-ui"></div>
			<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
			<script>
			window.onload = () => {
				window.ui = SwaggerUIBundle({
				url: '/openapi.json',
				dom_id: '#swagger-ui',
				});
			};
			</script>
			</body>
			</html>`))
}
