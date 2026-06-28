package api

// registerRoutes wires up all HTTP routes to their handler functions.
func (s *Server) registerRoutes() {

	// public routes — no auth required
	s.router.GET("/healthz", s.handleHealthz)
	s.router.POST("/auth/register", s.register)
	s.router.POST("/auth/login", s.login)

	// protected routes — JWT required
	v1 := s.router.Group("/api/v1")
	v1.Use(JWTMiddleware(s.jwtSecret))
	{
		// clusters
		v1.GET("/clusters", s.listClusters)
		v1.POST("/clusters", s.createCluster)
		v1.GET("/clusters/:id", s.getCluster)
		v1.DELETE("/clusters/:id", s.deleteCluster)

		// recommendations
		v1.GET("/clusters/:id/recommendations", s.listRecommendations)
	}
}
