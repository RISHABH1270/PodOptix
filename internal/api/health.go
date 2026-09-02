package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealthz responds to Kubernetes liveness probes.
// Always 200 — just confirms the process is alive, does not check dependencies.
func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleReadyz responds to Kubernetes readiness probes.
// Returns 200 only if PostgreSQL and Redis are reachable.
// Kubernetes stops routing traffic to this pod if readyz fails.
func (s *Server) handleReadyz(c *gin.Context) {
	checks := gin.H{}
	ready := true

	if err := s.store.Ping(c.Request.Context()); err != nil {
		checks["postgres"] = "error"
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if s.cache != nil {
		if err := s.cache.Ping(c.Request.Context()); err != nil {
			checks["redis"] = "error"
			ready = false
		} else {
			checks["redis"] = "ok"
		}
	}

	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "checks": checks})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": checks})
}
