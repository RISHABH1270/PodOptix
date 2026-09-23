package api

import (
	"strconv"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/metrics"
	"github.com/gin-gonic/gin"
)

// MetricsMiddleware records latency + count for every HTTP request.
// Skips /metrics itself to avoid feedback loops.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		// FullPath returns the matched route template (e.g. /api/v1/clusters/:id),
		// not the actual URL — keeps label cardinality bounded.
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
