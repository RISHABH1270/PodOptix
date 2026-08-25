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
	Router        *gin.Engine  // exported — allows test packages to call Router.ServeHTTP directly
	store         *store.Store
	cache         *cache.Cache
	jwtSecret     string
	encryptionKey string
}

// NewServer creates a new HTTP server and registers all routes.
func NewServer(st *store.Store, ca *cache.Cache, jwtSecret string, encryptionKey string) *Server {
	router := gin.Default()
	router.Use(RequestIDMiddleware())

	server := &Server{
		Router:        router,
		store:         st,
		cache:         ca,
		jwtSecret:     jwtSecret,
		encryptionKey: encryptionKey,
	}

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

// Serve starts accepting HTTP requests on the given listener. Blocking call.
func (s *Server) Serve(listener net.Listener) error {
	return s.Router.RunListener(listener)
}
