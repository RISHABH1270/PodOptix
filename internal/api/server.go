package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/RISHABH1270/PodOptix/internal/cache"
	"github.com/RISHABH1270/PodOptix/internal/scheduler"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router and all its dependencies.
type Server struct {
	router        *gin.Engine         // Gin router — knows all routes and middleware
	store         *store.Store        // database connection injected from main
	cache         *cache.Cache        // Redis cache injected from main
	scheduler     *scheduler.Scheduler // used to trigger immediate sync on cluster registration
	jwtSecret     string              // used to sign and verify JWT tokens
	encryptionKey string              // used to encrypt/decrypt Prometheus tokens at rest
}

// NewServer creates a new HTTP server and registers all routes.
func NewServer(st *store.Store, ca *cache.Cache, sched *scheduler.Scheduler, jwtSecret string, encryptionKey string) *Server {
	gin.SetMode(gin.ReleaseMode) // suppress debug route logs — not useful in production or tests
	router := gin.New()
	router.Use(gin.Recovery())    // keep panic recovery
	router.Use(RequestIDMiddleware())
	router.SetTrustedProxies(nil) // direct connection only — no reverse proxy trust

	server := &Server{
		router:        router,
		store:         st,
		cache:         ca,
		scheduler:     sched,
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
	return s.router.RunListener(listener)
}

// ServeHTTP implements http.Handler — used by httptest.NewServer in tests to start a real TCP listener.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
