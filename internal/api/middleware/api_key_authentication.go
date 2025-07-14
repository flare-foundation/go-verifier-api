package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func ApiKeyAuthMiddleware() gin.HandlerFunc {
	expectedAPIKey := os.Getenv("API_KEY")
	if expectedAPIKey == "" {
		panic("API_KEY is not set in environment")
	}

	return func(c *gin.Context) {
		providedAPIKey := c.GetHeader("X-API-KEY")
		if providedAPIKey != expectedAPIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or missing API key",
			})
			return
		}
		c.Next()
	}
}
