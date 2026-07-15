package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// FrontendFS holds the compiled React app embedded at build time.
// The //go:embed directive tells Go to include frontend/dist/ in the binary.
var FrontendFS embed.FS

// serveFrontend registers a catch-all route that serves the React SPA.
// Any URL not matched by API routes returns index.html — React Router handles client-side navigation from there.
func (s *Server) serveFrontend() {
	distFS, err := fs.Sub(FrontendFS, "frontend/dist")
	if err != nil {
		// frontend not embedded — development mode, skip
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	// serve static assets (JS, CSS, images)
	s.router.NoRoute(func(c *gin.Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
