package api

import (
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router and all its dependencies.
type Server struct {
	router *gin.Engine
	store  *store.Store // database connection injected from main
}

// NewServer creates a new HTTP server and registers all routes.
// store is injected from main.go — server does not create its own connection.
func NewServer(st *store.Store) *Server {
	// create a new gin router
	var router *gin.Engine
	router = gin.Default()

	// attach request ID middleware — runs before every request
	router.Use(RequestIDMiddleware())

	// create the server object with store injected
	var server *Server
	server = &Server{
		router: router,
		store:  st,
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
