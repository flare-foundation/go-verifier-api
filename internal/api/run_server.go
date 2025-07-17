package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/joho/godotenv"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
)

func RunServer() {
	_ = godotenv.Load()

	verifierTypeStr := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	sourceIdStr := os.Getenv("SOURCE_ID")
	if verifierTypeStr == "" || port == "" || sourceIdStr == "" {
		logger.Fatal("VERIFIER_TYPE, PORT and SOURCE_ID must be set")
	}
	verifierType, err := attestationutils.ParseAttestationType(verifierTypeStr)
	if err != nil {
		logger.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}
	_, err = attestationutils.ParseSourceId(sourceIdStr)
	if err != nil {
		logger.Fatalf("Invalid SOURCE_ID in .env: %v", err)
	}

	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("FTDC Verifier API", "1.0"))

	err = LoadModule(api, verifierType)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	fmt.Printf("Starting server on :%s...", port)
	logger.Fatal(http.ListenAndServe(":"+port, router))
}
