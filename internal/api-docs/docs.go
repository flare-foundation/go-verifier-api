package apidocs

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed swagger-ui/*
var swaggerFiles embed.FS

func SwaggerIndexHandler(w http.ResponseWriter, r *http.Request) {
	subFS, err := fs.Sub(swaggerFiles, "swagger-ui")
	if err != nil {
		http.Error(w, "Internal server error", 500)
		return
	}
	data, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		http.Error(w, "File not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:")
	w.Write(data)
}

func SwaggerFileHandler(w http.ResponseWriter, r *http.Request) {
	subFS, err := fs.Sub(swaggerFiles, "swagger-ui")
	if err != nil {
		http.Error(w, "Internal server error", 500)
		return
	}
	http.StripPrefix("/api-doc/", http.FileServer(http.FS(subFS))).ServeHTTP(w, r)
}
