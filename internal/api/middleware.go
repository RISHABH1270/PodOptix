package api

import (
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
