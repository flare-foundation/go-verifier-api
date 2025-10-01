package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

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

const (
	EnvDevelopment    = "development"
	shutdownAfter     = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
)

func RunServer(envConfig config.EnvConfig) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, closers := startServer(ctx, envConfig)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownServer(srv, closers)
}

func startServer(ctx context.Context, envConfig config.EnvConfig) (*http.Server, []io.Closer) {
	router := newRouter()
	api := newAPI(router, envConfig)

	closers, err := LoadModule(ctx, api, envConfig)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	srv := &http.Server{
		Addr:              ":" + envConfig.Port,
		Handler:           newSecurityHandler(envConfig, router),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		logger.Infof("Starting %s verifier server with type %s on: %s ...\n",
			envConfig.SourceID, envConfig.AttestationType, envConfig.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	return srv, closers
}

func shutdownServer(srv *http.Server, closers []io.Closer) {
	logger.Info("Shutting down gracefully...")

	for _, c := range closers {
		if err := c.Close(); err != nil {
			logger.Errorf("Error closing service: %v", err)
		}
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), shutdownAfter)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}
}

func RunServerForTest(t *testing.T, envConfig config.EnvConfig) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	srv, closers := startServer(ctx, envConfig)

	stop = func() {
		cancel()
		shutdownServer(srv, closers)
	}

	return stop
}

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
	connector.PMWMultisigAccountConfigured,
}

var SourceIDs = []config.SourceName{
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

func parseSourceID(value string) (config.SourceName, error) {
	for _, at := range SourceIDs {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid source id: %s", value)
}

func getAPIKeys() ([]string, error) {
	raw := os.Getenv(config.EnvAPIKeys)
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s must be set", config.EnvAPIKeys)
	}
	var apiKeys []string
	for _, key := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			apiKeys = append(apiKeys, trimmed)
		}
	}
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("%s contains only empty values", config.EnvAPIKeys)
	}
	return apiKeys, nil
}

func LoadEnvConfig() (config.EnvConfig, error) {
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file found, proceeding with environment variables")
	}
	port, err := getEnvOrError(config.EnvPort)
	if err != nil {
		return config.EnvConfig{}, err
	}
	verifierTypeStr, err := getEnvOrError(config.EnvAttestationType)
	if err != nil {
		return config.EnvConfig{}, err
	}
	sourceIDStr, err := getEnvOrError(config.EnvSourceID)
	if err != nil {
		return config.EnvConfig{}, err
	}
	attestationType, err := parseAttestationType(verifierTypeStr)
	if err != nil {
		return config.EnvConfig{}, err
	}
	sourceID, err := parseSourceID(sourceIDStr)
	if err != nil {
		return config.EnvConfig{}, err
	}
	apiKeys, err := getAPIKeys()
	if err != nil {
		return config.EnvConfig{}, err
	}
	env := os.Getenv(config.EnvEnv)
	if env == "" {
		logger.Infof("%s is not set, defaulting to development", config.EnvEnv)
		env = EnvDevelopment
	}
	return config.EnvConfig{
		RPCURL:                            os.Getenv(config.EnvRPCURL),
		RelayContractAddress:              os.Getenv(config.EnvRelayContractAddress),
		TeeMachineRegistryContractAddress: os.Getenv(config.EnvTeeMachineRegistryContractAddress),
		SourceDatabaseURL:                 os.Getenv(config.EnvSourceDatabaseURL),
		CChainDatabaseURL:                 os.Getenv(config.EnvCChainDatabaseURL),
		AllowTeeDebug:                     os.Getenv(config.EnvAllowTeeDebug),
		DisableAttestationCheckE2E:        os.Getenv(config.EnvDisableAttestationCheckE2E),
		Env:                               env,
		Port:                              port,
		APIKeys:                           apiKeys,
		AttestationType:                   attestationType,
		SourceID:                          sourceID,
	}, nil
}

func getEnvOrError(key string) (string, error) {
	val := os.Getenv(key)
	val = strings.TrimSpace(val)
	if val == "" {
		return "", fmt.Errorf("%s must be set", key)
	}
	return val, nil
}

func newRouter() chi.Router {
	router := chi.NewRouter()
	router.Get("/api-doc", apidocs.SwaggerIndexHandler)
	router.Get("/api-doc/*", apidocs.SwaggerFileHandler)
	return router
}

func newAPI(router chi.Router, envConfig config.EnvConfig) huma.API {
	cfg := huma.DefaultConfig("FTDC Verifier API", "1.0")
	cfg.Info.Description = fmt.Sprintf("The Flare TEE Data Connector Verifier API endpoints for %s attestation sourced from %s.", envConfig.AttestationType, envConfig.SourceID)
	cfg.DocsPath = ""
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"ApiKeyAuth": {
			Type: "apiKey",
			In:   "header",
			Name: "X-API-KEY",
		},
	}
	cfg.Security = []map[string][]string{
		{"ApiKeyAuth": {}},
	}

	api := humachi.New(router, cfg)
	api.UseMiddleware(middleware.APIKeyAuthMiddleware(api, envConfig.APIKeys))
	return api
}

func newSecurityHandler(envConfig config.EnvConfig, handler http.Handler) http.Handler {
	const (
		SecondsPerDay        = 24 * 60 * 60
		STSDurationInSeconds = 180 * SecondsPerDay
	)
	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:               envConfig.Env != EnvDevelopment,
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
		IsDevelopment:             envConfig.Env == EnvDevelopment,
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	return corsHandler.Handler(secureMiddleware.Handler(handler))
}
