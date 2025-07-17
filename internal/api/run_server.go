package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/joho/godotenv"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/type"
)

func RunServer() {
	_ = godotenv.Load()

	verifierTypeStr := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	sourceIdStr := os.Getenv("SOURCE_ID")
	if verifierTypeStr == "" || port == "" || sourceIdStr == "" {
		logger.Fatal("VERIFIER_TYPE, PORT and SOURCE_ID must be set")
	}
	verifierType, err := parseAttestationType(verifierTypeStr)
	if err != nil {
		logger.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}
	_, err = parseSourceId(sourceIdStr)
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

func parseSourceId(value string) (attestationtypes.SourceName, error) {
	for _, at := range sourceIds {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

var sourceIds = []attestationtypes.SourceName{
	attestationtypes.SourceTEE,
	attestationtypes.SourceXRP,
}
