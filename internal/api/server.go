package api

import (
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router and all its dependencies.
type Server struct {
	router *gin.Engine
}

// NewServer creates a new HTTP server and registers all routes.
func NewServer() *Server {
	// create a new gin router
	var router *gin.Engine
	router = gin.Default()

	// create the server object
	var server *Server
	server = &Server{
		router: router,
	}

	// register all routes on the router
	server.registerRoutes()

	return server
}

// Start begins listening for incoming HTTP requests on the given port.
// This is a blocking call — the app stays running here until stopped.
func (s *Server) Start(port string) error {
	return s.router.Run(":" + port)
}
