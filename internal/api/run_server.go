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

	err := LoadModule(api, envConfig)
	if err != nil {
		logger.Fatalf("%v", err)
	}
	const (
		SecondsPerDay        = 24 * 60 * 60
		STSDurationInSeconds = 180 * SecondsPerDay
	)
	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:               envConfig.Env != "development",
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
		IsDevelopment:             envConfig.Env == "development", // TODO can this be handled in a better way?
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

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
	connector.PMWMultisigAccountConfigured,
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
	raw := os.Getenv(config.EnvApiKeys)
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s must be set", config.EnvApiKeys)
	}
	var apiKeys []string
	for _, key := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			apiKeys = append(apiKeys, trimmed)
		}
	}
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("%s contains only empty values", config.EnvApiKeys)
	}
	return apiKeys, nil
}

func LoadEnvConfig() (config.EnvConfig, error) {
	err := godotenv.Load()
	if err != nil {
		logger.Warn("No .env file found, proceeding with environment variables")
	}
	port := os.Getenv(config.EnvPort)
	if port == "" {
		return config.EnvConfig{}, fmt.Errorf("%s must be set", config.EnvPort)
	}
	verifierTypeStr := os.Getenv(config.EnvAttestationType)
	if verifierTypeStr == "" {
		return config.EnvConfig{}, fmt.Errorf("%s must be set", config.EnvAttestationType)
	}
	sourceIDStr := os.Getenv(config.EnvSourceID)
	if sourceIDStr == "" {
		return config.EnvConfig{}, fmt.Errorf("%s must be set", config.EnvSourceID)
	}
	attestationType, err := parseAttestationType(verifierTypeStr)
	if err != nil {
		logger.Fatalf("Invalid %s: %v", config.EnvAttestationType, err)
	}
	sourceID, err := parseSourceId(sourceIDStr)
	if err != nil {
		logger.Fatalf("Invalid %s: %v", config.EnvSourceID, err)
	}
	apiKeys, err := getAPIKeys()
	if err != nil {
		logger.Fatalf("%v", err)
	}
	env := os.Getenv(config.EnvEnv)
	if env == "" {
		logger.Warnf("%s is not set, defaulting to development", config.EnvEnv)
		env = "development"
	}

	return config.EnvConfig{
		RPCURL:                                 os.Getenv(config.EnvRPCURL),
		RelayContractAddress:                   os.Getenv(config.EnvRelayContractAddress),
		TeeMachineRegistryContractAddress:      os.Getenv(config.EnvTeeMachineRegistryContractAddress),
		TeeWalletManagerContractAddress:        os.Getenv(config.EnvTeeWalletManagerContractAddress),
		TeeWalletProjectManagerContractAddress: os.Getenv(config.EnvTeeWalletProjectManagerContractAddress),
		DatabaseURL:                            os.Getenv(config.EnvDatabaseURL),
		CChainDatabaseURL:                      os.Getenv(config.EnvCChainDatabaseURL),
		Env:                                    env,
		Port:                                   port,
		ApiKeys:                                apiKeys,
		AttestationType:                        attestationType,
		SourceID:                               sourceID,
	}, nil
}
