package recommender

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/stretchr/testify/assert"
)

// ── test counter ──────────────────────────────────────────────────────────────

var (
	passed, failed, counter int
	mu                      sync.Mutex
	tty                     *os.File
)

func init() {
	var err error
	tty, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = os.Stderr
	}
}

func log(format string, args ...any) { fmt.Fprintf(tty, format, args...) }

func shortName(full string) string {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}

func track(t *testing.T) {
	t.Helper()
	mu.Lock()
	counter++
	n := counter
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if t.Failed() {
			failed++
			log("  [%2d]  ✗  %s\n", n, shortName(t.Name()))
		} else {
			passed++
			log("  [%2d]  ✓  %s\n", n, shortName(t.Name()))
		}
	})
}

func TestMain(m *testing.M) {
	log("\n  Running Recommender Tests...\n")
	log("  ──────────────────────────────────────\n")
	code := m.Run()
	log("  Total: %d  |  Passed: %d  |  Failed: %d\n", passed+failed, passed, failed)
	if code == 0 {
		log("  ✓ All tests passed\n")
	} else {
		log("  ✗ Some tests failed\n")
	}
	log("  ──────────────────────────────────────\n\n")
	os.Exit(code)
}

// ── generate ──────────────────────────────────────────────────────────────────

func TestGenerate(t *testing.T) {
	t.Run("success — recommended is ceil(p99 × 2)", func(t *testing.T) {
		track(t)
		metrics := &collector.ContainerMetrics{
			Namespace: "payments", PodName: "payment-api", ContainerName: "api",
			CPUValues: []float64{100, 110, 120, 105, 115}, // p99 = 120 → recommended = 240
			MemValues: []float64{200, 210, 220, 205, 215}, // p99 = 220 → recommended = 440
			CPULimit:  1000,
			MemLimit:  1024,
		}
		rec, err := generate("cluster-123", metrics)
		assert.NoError(t, err)
		assert.Equal(t, "cluster-123", rec.ClusterID)
		assert.Equal(t, "payments", rec.Namespace)
		assert.Equal(t, "payment-api", rec.PodName)
		assert.Equal(t, "api", rec.ContainerName)
		assert.Equal(t, models.RecommendationStatusReady, rec.Status)
		assert.Equal(t, 1000, rec.CurrentCPULimit)
		assert.Equal(t, 1024, rec.CurrentMemLimit)
		assert.Equal(t, 240, rec.RecommendedCPULimit)
		assert.Equal(t, 440, rec.RecommendedMemLimit)
		assert.NotEmpty(t, rec.RecommendationID)
	})

	t.Run("nil metrics returns error", func(t *testing.T) {
		track(t)
		_, err := generate("cluster-123", nil)
		assert.ErrorContains(t, err, "metrics cannot be nil")
	})

	t.Run("empty CPU values returns error", func(t *testing.T) {
		track(t)
		_, err := generate("cluster-123", &collector.ContainerMetrics{
			CPUValues: []float64{},
			MemValues: []float64{200, 210},
		})
		assert.ErrorContains(t, err, "compute p99 cpu")
	})

	t.Run("empty memory values returns error", func(t *testing.T) {
		track(t)
		_, err := generate("cluster-123", &collector.ContainerMetrics{
			CPUValues: []float64{100, 110},
			MemValues: []float64{},
		})
		assert.ErrorContains(t, err, "compute p99 mem")
	})

	t.Run("single value — recommended is double", func(t *testing.T) {
		track(t)
		rec, err := generate("cluster-1", &collector.ContainerMetrics{
			Namespace: "ns", PodName: "pod", ContainerName: "c",
			CPUValues: []float64{50},
			MemValues: []float64{100},
		})
		assert.NoError(t, err)
		assert.Equal(t, 100, rec.RecommendedCPULimit) // 50 × 2
		assert.Equal(t, 200, rec.RecommendedMemLimit) // 100 × 2
	})
}

// ── GenerateAll ───────────────────────────────────────────────────────────────

func TestGenerateAll(t *testing.T) {
	t.Run("mixed data — ready and new_service", func(t *testing.T) {
		track(t)
		recs, err := GenerateAll("cluster-1", []*collector.ContainerMetrics{
			{
				Namespace: "ns", PodName: "pod-1", ContainerName: "c1",
				CPUValues: []float64{100, 110, 120},
				MemValues: []float64{200, 210, 220},
			},
			{
				Namespace: "ns", PodName: "pod-2", ContainerName: "c2",
				CPUValues: []float64{}, // no data — new service
				MemValues: []float64{},
			},
		})
		assert.NoError(t, err)
		assert.Len(t, recs, 2)
		assert.Equal(t, models.RecommendationStatusReady, recs[0].Status)
		assert.Equal(t, models.RecommendationStatusNewService, recs[1].Status)
	})

	t.Run("empty metrics returns empty slice", func(t *testing.T) {
		track(t)
		recs, err := GenerateAll("cluster-1", []*collector.ContainerMetrics{})
		assert.NoError(t, err)
		assert.Empty(t, recs)
	})
}
