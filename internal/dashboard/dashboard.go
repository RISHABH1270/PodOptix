// Package dashboard embeds the built React dashboard (web/dist) into the Go binary
// and serves it over HTTP alongside the REST API — single-binary deployment.
//
// Build workflow:
//   1. cd web && npm run build   →  outputs to internal/dashboard/dist
//   2. go build ./cmd/hub        →  //go:embed pulls dist/ into the binary
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// distFS holds every file under internal/dashboard/dist at compile time.
// The `all:` prefix includes files starting with `.` and `_` (e.g. .gitkeep).
//
//go:embed all:dist
var distFS embed.FS

// Handler returns an HTTP handler that serves the embedded dashboard.
//   - Existing static files (JS, CSS, images, favicon) → served directly with correct MIME type.
//   - Unknown paths (React Router routes like /clusters/:id) → served index.html so the SPA can take over.
//   - Empty embed (dashboard not yet built) → 503 with a helpful hint.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("dashboard: fs.Sub failed: " + err.Error())
	}

	// One-time check: is index.html present? If not, the dashboard was never built.
	indexBytes, _ := fs.ReadFile(sub, "index.html")
	built := len(indexBytes) > 0
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !built {
			http.Error(w,
				"Dashboard not built.\n\nRun: cd web && npm run build\nThen restart the server.\n",
				http.StatusServiceUnavailable)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Static file exists → serve it (correct MIME, cache headers, etc. via http.FileServer)
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: any unknown path returns index.html so React Router can handle it.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexBytes)
	})
}
