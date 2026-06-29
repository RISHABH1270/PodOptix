package api

import (
	"net/http"
	"strings"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware assigns a unique request ID to every incoming request.
// The ID is available to all handlers via the context and returned in the response header.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// generate unique ID for this request
		var requestID string
		requestID = uuid.New().String()

		// store it in context so handlers can read it
		c.Set("request_id", requestID)

		// add it to response header — standard practice (X-Request-ID)
		c.Header("X-Request-ID", requestID)

		// pass to next handler
		c.Next()
	}
}

// JWTMiddleware verifies the JWT token on every protected route.
// Rejects with 401 if token is missing, expired, or tampered.
func JWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestID string
		requestID = c.GetString("request_id")

		// read Authorization header — expected format: "Bearer <token>"
		var authHeader string
		authHeader = c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authorization header is required",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// split into ["Bearer", "<token>"]
		var parts []string
		parts = strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authorization header format must be: Bearer <token>",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// validate the token
		claims, err := auth.ValidateToken(parts[1], secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Invalid or expired token",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// store user info in context — available to all handlers
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)

		c.Next()
	}
}
