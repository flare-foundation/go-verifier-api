package api

import (
	"log"
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
	if verifierTypeStr == "" || port == "" {
		log.Fatal("VERIFIER_TYPE and PORT must be set")
	}
	verifierType, err := attestationutils.ParseAttestationType(verifierTypeStr)
	if err != nil {
		log.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}

	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Verifier API", "1.0"))

	err = LoadModule(api, verifierType)
	if err != nil {
		logger.Fatalf("Failed to load verifier module: %v", err)
	}

	log.Printf("Starting server on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
