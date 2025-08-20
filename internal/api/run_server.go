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
	envConfig, err := loadEnvConfig()
	if err != nil {
		logger.Fatal(err)
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
	api.UseMiddleware(middleware.APIKeyAuthMiddleware(api, envConfig.ApiKeys))

	// swagger setup
	router.Get("/api-doc", apidocs.SwaggerIndexHandler)
	router.Get("/api-doc/*", apidocs.SwaggerFileHandler)

	err = LoadModule(api, envConfig)
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
		IsDevelopment:             envConfig.Env == "development", // TODO can this be handled in a better way?
	})
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})
	routerWithSecurity := secureMiddleware.Handler(router)
	routerWithCORS := corsHandler.Handler(routerWithSecurity)

	fmt.Printf("Starting server on: %s...\n", envConfig.Port)
	logger.Fatal(http.ListenAndServe(": "+envConfig.Port, routerWithCORS))
}

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
	connector.PMWMultisigAccountConfigured,
}

var sourceIds = []config.SourceName{
	config.SourceTEE,
	config.SourceXRP,
	config.SourceTestXRP,
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

func loadEnvConfig() (config.EnvConfig, error) {
	err := godotenv.Load()
	if err != nil {
		return config.EnvConfig{}, fmt.Errorf("error loading .env file: %v", err)
	}
	port := os.Getenv("PORT")
	verifierTypeStr := os.Getenv("VERIFIER_TYPE")
	sourceIDStr := os.Getenv("SOURCE_ID")
	if port == "" || verifierTypeStr == "" || sourceIDStr == "" {
		return config.EnvConfig{}, fmt.Errorf("PORT, VERIFIER_TYPE and SOURCE_ID must be set")
	}
	attestationType, err := parseAttestationType(verifierTypeStr)
	if err != nil {
		logger.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}
	sourceID, err := parseSourceId(sourceIDStr)
	if err != nil {
		logger.Fatalf("Invalid SOURCE_ID in .env: %v", err)
	}
	apiKeys, err := getAPIKeys()
	if err != nil {
		logger.Fatalf("%v", err)
	}

	env := os.Getenv("ENV")
	if env == "" {
		logger.Warn("ENV is not set, defaulting to production")
		env = "production"
	}

	return config.EnvConfig{
		RPCURL:                                 os.Getenv("RPC_URL"),
		RelayContractAddress:                   os.Getenv("RELAY_CONTRACT_ADDRESS"),
		TeeRegistryContractAddress:             os.Getenv("TEE_REGISTRY_CONTRACT_ADDRESS"),
		TeeWalletManagerContractAddress:        os.Getenv("TEE_WALLET_MANAGER_CONTRACT_ADDRESS"),
		TeeWalletProjectManagerContractAddress: os.Getenv("TEE_WALLET_PROJECT_MANAGER_CONTRACT_ADDRESS"),
		DatabaseURL:                            os.Getenv("DATABASE_URL"),
		CChainDatabaseURL:                      os.Getenv("CCHAIN_DATABASE_URL"),
		Env:                                    env,
		Port:                                   port,
		ApiKeys:                                apiKeys,
		AttestationType:                        attestationType,
		SourceID:                               sourceID,
	}, nil
}
