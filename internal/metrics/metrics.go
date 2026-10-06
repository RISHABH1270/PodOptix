// Package metrics exposes Prometheus metrics for PodOptix.
// Lives in its own package (not internal/api) so packages like scheduler
// can import metrics without creating a cycle with api.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "podoptix",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests processed, labelled by method, path template, status.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "podoptix",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds, labelled by method and path template.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})

	SchedulerRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "podoptix",
		Subsystem: "scheduler",
		Name:      "runs_total",
		Help:      "Scheduler runs per cluster, labelled by outcome (success | failure).",
	}, []string{"outcome"})

	SchedulerRunDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "podoptix",
		Subsystem: "scheduler",
		Name:      "run_duration_seconds",
		Help:      "Time taken for a scheduler run over all clusters.",
		Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
	})

	SchedulerContainersScanned = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "podoptix",
		Subsystem: "scheduler",
		Name:      "containers_scanned_total",
		Help:      "Cumulative count of containers scanned by the scheduler.",
	})

	CacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "podoptix",
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "Redis cache hits, labelled by key type.",
	}, []string{"kind"})

	CacheMissesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "podoptix",
		Subsystem: "cache",
		Name:      "misses_total",
		Help:      "Redis cache misses, labelled by key type.",
	}, []string{"kind"})
)
