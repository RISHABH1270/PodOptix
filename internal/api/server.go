package api

import (
	"fmt"
	"net"

	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router and all its dependencies.
type Server struct {
	router *gin.Engine
	store  *store.Store // database connection injected from main
}

// Constructor - NewServer creates a new HTTP server and registers all routes.
func NewServer(st *store.Store) *Server {
	// create a new gin router
	var router *gin.Engine
	router = gin.Default()

	// attach request ID middleware
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

// Listen binds the TCP port. Returns the listener if successful.
func (s *Server) Listen(port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("bind port %s: %w", port, err)
	}
	return listener, nil
}

// Serve starts accepting HTTP requests on the given listener.
// Blocking call — returns only on error.
func (s *Server) Serve(listener net.Listener) error {
	return s.router.RunListener(listener)
}
