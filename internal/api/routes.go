package api

// registerRoutes wires up all HTTP routes to their handler functions.
func (s *Server) registerRoutes() {

	// public routes — no auth required
	s.Router.GET("/healthz", s.handleHealthz)  // liveness  — is process alive?
	s.Router.GET("/readyz", s.handleReadyz)    // readiness — are dependencies ready?
	s.Router.POST("/auth/register", s.register)
	s.Router.POST("/auth/login", s.login)

	// protected routes — JWT required
	v1 := s.Router.Group("/api/v1")
	v1.Use(JWTMiddleware(s.jwtSecret))
	{
		// clusters
		v1.GET("/clusters", s.listClusters)
		v1.POST("/clusters", s.createCluster)
		v1.GET("/clusters/:id", s.getCluster)
		v1.PUT("/clusters/:id", s.updateCluster)
		v1.DELETE("/clusters/:id", s.deleteCluster)

		// recommendations
		v1.GET("/clusters/:id/recommendations", s.listRecommendations)
		v1.POST("/clusters/:id/recalculate", s.recalculate)
	}
}
