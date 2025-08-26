package apidocs

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

//go:embed swagger-ui/*
var swaggerFiles embed.FS

func SwaggerIndexHandler(w http.ResponseWriter, r *http.Request) {
	subFS, err := fs.Sub(swaggerFiles, "swagger-ui")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	data, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:")
	_, err = w.Write(data)
	if err != nil {
		logger.Error("Failed to write response: %v", err)
		return
	}
}

func SwaggerFileHandler(w http.ResponseWriter, r *http.Request) {
	subFS, err := fs.Sub(swaggerFiles, "swagger-ui")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/api-doc/", http.FileServer(http.FS(subFS))).ServeHTTP(w, r)
}
