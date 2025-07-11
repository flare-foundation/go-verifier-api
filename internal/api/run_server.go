package api

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "gitlab.com/urskak/verifier-api/internal/api/docs"
)

func RunServer() {
	_ = godotenv.Load()

	verifierType := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	if verifierType == "" || port == "" {
		log.Fatal("VERIFIER_TYPE and PORT must be set")
	}

	router := gin.Default()

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	err := LoadModule(router, verifierType)
	if err != nil {
		log.Fatalf("Failed to load verifier module: %v", err)
	}

	log.Printf("Starting server on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
