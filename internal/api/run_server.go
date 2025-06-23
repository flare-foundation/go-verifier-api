package api

import (
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func RunServer() {
	_ = godotenv.Load()

	verifierType := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	if verifierType == "" || port == "" {
		log.Fatal("VERIFIER_TYPE and PORT must be set")
	}

	router := chi.NewRouter()
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	config := huma.DefaultConfig("verifier-api", "1.0.0")
	config.Components.Schemas = registry

	app := humachi.New(router, config)

	err := LoadModule(verifierType, app, registry)
	if err != nil {
		log.Fatalf("Failed to load verifier module: %v", err)
	}

	router.Get("/api-doc", func(w http.ResponseWriter, r *http.Request) {
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

	router.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html>
	<html>
	  <head>
		<title>API Reference</title>
		<meta charset="utf-8" />
		<meta
		  name="viewport"
		  content="width=device-width, initial-scale=1" />
	  </head>
	  <body>
		<script
		  id="api-reference"
		  data-url="/openapi.json"></script>
		<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
	  </body>
	</html>`))
	})

	log.Printf("Starting server on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
