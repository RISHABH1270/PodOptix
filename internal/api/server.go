package api

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/RISHABH1270/PodOptix/internal/cache"
	"github.com/RISHABH1270/PodOptix/internal/scheduler"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/gin-gonic/gin"
)

// Server holds the HTTP router + the underlying *http.Server (needed for graceful Shutdown)
// and the dependencies the handlers need.
type Server struct {
	router        *gin.Engine          // Gin router — knows all routes and middleware
	httpServer    *http.Server         // owns the lifecycle — Serve blocks on it, Shutdown drains it
	store         *store.Store         // database connection injected from main
	cache         *cache.Cache         // Redis cache injected from main
	scheduler     *scheduler.Scheduler // used to trigger immediate sync on cluster registration
	jwtSecret     string               // used to sign and verify JWT tokens
	encryptionKey string               // used to encrypt/decrypt Prometheus tokens at rest
}

// NewServer creates a new HTTP server and registers all routes.
func NewServer(st *store.Store, ca *cache.Cache, sched *scheduler.Scheduler, jwtSecret string, encryptionKey string) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware())
	router.Use(MetricsMiddleware()) // records latency + count per route into Prometheus metrics
	router.SetTrustedProxies(nil)   // **direct connection only — no reverse proxy trust**

	server := &Server{
		router:        router,
		httpServer:    &http.Server{Handler: router},
		store:         st,
		cache:         ca,
		scheduler:     sched,
		jwtSecret:     jwtSecret,
		encryptionKey: encryptionKey,
	}

	server.registerRoutes()

	return server
}

// Listen binds the TCP port.
// Pre-flights both loopbacks because net.Listen(":PORT") would silently coexist
// with a process already bound to a specific-address loopback (e.g. Jenkins
// on [::1]:8080), splitting connections between us and them.
func (s *Server) Listen(port string) (net.Listener, error) {
	for _, addr := range []string{"127.0.0.1:" + port, "[::1]:" + port} {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("port %s is already in use on %s: %w", port, addr, err)
		}
		l.Close()
	}
	return net.Listen("tcp", ":"+port)
}

// Serve starts accepting HTTP requests on the given listener. Blocking call.
// Returns http.ErrServerClosed after a graceful Shutdown — callers should treat that as success.
func (s *Server) Serve(listener net.Listener) error {
	return s.httpServer.Serve(listener)
}

// Shutdown stops accepting new connections and waits for in-flight handlers to finish
// (up to ctx's deadline). Returns context.DeadlineExceeded if a handler is still running
// when the deadline hits.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// ServeHTTP implements http.Handler — used by httptest.NewServer in tests to start a real TCP listener.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
