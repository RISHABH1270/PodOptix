package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealthz responds to Kubernetes liveness probes.
// Returns 200 OK if the server is running.
func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
