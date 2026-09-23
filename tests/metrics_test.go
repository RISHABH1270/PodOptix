package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	t.Run("GET /metrics", func(t *testing.T) {
		t.Run("public — no auth required", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/metrics", "", "")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
		})

		t.Run("exposes podoptix_ metrics", func(t *testing.T) {
			track(t)
			// fire a request first so http_requests_total has data
			do(t, http.MethodGet, "/healthz", "", "").Body.Close()

			resp := do(t, http.MethodGet, "/metrics", "", "")
			body := readBody(t, resp)
			// Only non-vec counters and metrics with at least one label combination emit
			// output. HTTP metrics are guaranteed here because we just fired /healthz.
			assert.True(t, strings.Contains(body, "podoptix_http_requests_total"),
				"expected podoptix_http_requests_total in metrics output")
			assert.True(t, strings.Contains(body, "podoptix_http_request_duration_seconds"),
				"expected podoptix_http_request_duration_seconds histogram")
			assert.True(t, strings.Contains(body, "podoptix_scheduler_containers_scanned_total"),
				"expected non-vec counter to emit even at zero")
		})

		t.Run("http_requests_total records the matched route template, not the raw URL", func(t *testing.T) {
			track(t)
			// hit a parameterised route so we can confirm the label is the template, not the ID
			id := createCluster(t, "metrics-route-cluster", "http://prom.metrics.test")
			do(t, http.MethodGet, "/api/v1/clusters/"+id, "", bearer(testToken())).Body.Close()

			body := readBody(t, do(t, http.MethodGet, "/metrics", "", ""))
			// route template with :id — NOT the literal UUID
			assert.Contains(t, body, `path="/api/v1/clusters/:id"`)
			assert.NotContains(t, body, `path="/api/v1/clusters/`+id+`"`,
				"raw UUID must never appear as a metric label — would blow up cardinality")
		})
	})
}
