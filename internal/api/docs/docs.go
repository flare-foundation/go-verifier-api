package apidocs

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

//go:embed swagger-ui/*
var swaggerFiles embed.FS

// SwaggerHandler serves the embedded Swagger UI mounted at basePath — the
// deployment-prefixed docs path with a trailing slash. The UI's references are
// relative, so any prefix works.
func SwaggerHandler(basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subFS, err := fs.Sub(swaggerFiles, "swagger-ui")
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == basePath || r.URL.Path == basePath+"index.html" {
			data, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:")
			if _, err := w.Write(data); err != nil {
				logger.Errorf("Failed to write response: %v", err)
			}
			return
		}
		http.StripPrefix(basePath, http.FileServer(http.FS(subFS))).ServeHTTP(w, r)
	}
}
