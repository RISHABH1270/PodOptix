package api

import (
	"net/http"
	"strings"

	"github.com/RISHABH1270/PodOptix/internal/dashboard"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registerRoutes wires up all HTTP routes to their handler functions.
func (s *Server) registerRoutes() {

	// public routes — no auth required
	s.router.GET("/healthz", s.handleHealthz)  // liveness  — is process alive?
	s.router.GET("/readyz", s.handleReadyz)    // readiness — are dependencies ready?
	s.router.POST("/auth/register", s.register)
	s.router.POST("/auth/login", s.login)

	// Prometheus scrape target — public because scrapers usually run without auth.
	// Restrict via NetworkPolicy if scraping from outside the cluster.
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// protected routes — JWT required
	v1 := s.router.Group("/api/v1")
	v1.Use(JWTMiddleware(s.jwtSecret))
	{
		// clusters
		v1.GET("/clusters", s.listClusters)
		v1.POST("/clusters", s.createCluster)
		v1.GET("/clusters/:id", s.getCluster)
		v1.PUT("/clusters/:id", s.updateCluster)
		v1.DELETE("/clusters/:id", s.deleteCluster)

		// recommendations
		v1.GET("/recommendations", s.listAllRecommendations)   // cross-cluster overview
		v1.GET("/clusters/:id/recommendations", s.listRecommendations)
		v1.POST("/clusters/:id/recalculate", s.recalculate)
	}

	// Any unmatched route → the embedded React dashboard (SPA).
	// API paths that don't exist still return 404 so clients see a real error.
	dashboardHandler := dashboard.Handler()
	s.router.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/auth/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found", "path": p})
			return
		}
		dashboardHandler.ServeHTTP(c.Writer, c.Request)
	})
}
