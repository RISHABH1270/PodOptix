package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealthz responds to Kubernetes liveness probes.
// Returns 200 OK if the process is alive — does not check dependencies.
func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// handleReadyz responds to Kubernetes readiness probes.
// Returns 200 only if all dependencies (PostgreSQL, Redis) are reachable.
// Kubernetes stops sending traffic to this pod if readyz fails.
func (s *Server) handleReadyz(c *gin.Context) {
	var checks = gin.H{}
	var allReady = true

	// check PostgreSQL
	if err := s.store.Ping(c.Request.Context()); err != nil {
		checks["postgres"] = "unhealthy"
		allReady = false
	} else {
		checks["postgres"] = "healthy"
	}

	// check Redis
	if s.cache != nil {
		if err := s.cache.Ping(c.Request.Context()); err != nil {
			checks["redis"] = "unhealthy"
			allReady = false
		} else {
			checks["redis"] = "healthy"
		}
	}

	if !allReady {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"checks": checks,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"checks": checks,
	})
}
