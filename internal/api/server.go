package api

import (
	"fmt"
	"net"

	"github.com/RISHABH1270/PodOptix/internal/cache"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router and all its dependencies.
type Server struct {
	router        *gin.Engine
	store         *store.Store // database connection injected from main
	cache         *cache.Cache // Redis cache injected from main
	jwtSecret     string       // used to sign and verify JWT tokens
	encryptionKey string       // used to encrypt/decrypt cluster tokens at rest
}

// NewServer creates a new HTTP server and registers all routes.
func NewServer(st *store.Store, ca *cache.Cache, jwtSecret string, encryptionKey string) *Server {
	var router *gin.Engine
	router = gin.Default()

	router.Use(RequestIDMiddleware())

	var server *Server
	server = &Server{
		router:        router,
		store:         st,
		cache:         ca,
		jwtSecret:     jwtSecret,
		encryptionKey: encryptionKey,
	}

	server.registerRoutes()
	server.serveFrontend() // serve embedded React app for all non-API routes

	return server
}

// Listen binds the TCP port. Returns the listener if successful.
// Caller can print "server is up" after this returns without error.
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
