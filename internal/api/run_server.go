package api

import (
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/go-chi/chi/v5"
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

	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("Verifier API", "1.0"))

	router.Get("/docs", func(w http.ResponseWriter, r *http.Request) { // https://huma.rocks/features/api-docs/
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
	})

	err = LoadModule(api, verifierType)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	log.Printf("Starting server on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
