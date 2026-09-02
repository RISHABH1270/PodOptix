package api

import (
	"net/http"
	"strings"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware assigns a unique UUID to every request.
// Stored in context as "request_id" and returned in X-Request-ID response header.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// JWTMiddleware verifies the Bearer token on every protected route.
// Rejects with 401 if token is missing, malformed, expired, or tampered.
// On success, sets "user_id" and "email" in the Gin context for downstream handlers.
func JWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetString("request_id")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required", "request_id": requestID})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be: Bearer <token>", "request_id": requestID})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(parts[1], secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token", "request_id": requestID})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}
