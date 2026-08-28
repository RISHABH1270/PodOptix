package compute

import (
	"fmt"
	"os"
	"sync"
	"testing"

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
	log("\n  Running Compute Tests...\n")
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

// ── ComputeP99 ────────────────────────────────────────────────────────────────

func TestComputeP99(t *testing.T) {
	t.Run("empty dataset returns error", func(t *testing.T) {
		track(t)
		_, err := ComputeP99([]float64{})
		assert.ErrorContains(t, err, "empty dataset")
	})

	t.Run("single value returns that value", func(t *testing.T) {
		track(t)
		result, err := ComputeP99([]float64{120.5})
		assert.NoError(t, err)
		assert.Equal(t, 120.5, result)
	})

	t.Run("two values returns highest", func(t *testing.T) {
		track(t)
		// ceil(0.99 × 2) = 2, index = 1 → 200.0
		result, err := ComputeP99([]float64{100.0, 200.0})
		assert.NoError(t, err)
		assert.Equal(t, 200.0, result)
	})

	t.Run("correct index position", func(t *testing.T) {
		track(t)
		// sorted: [1,2,3,4,5,6,7,8,9,10] — ceil(0.99×10)=10, index=9 → 10.0
		result, err := ComputeP99([]float64{5, 3, 8, 1, 9, 2, 7, 4, 10, 6})
		assert.NoError(t, err)
		assert.Equal(t, 10.0, result)
	})

	t.Run("top 1% spike ignored", func(t *testing.T) {
		track(t)
		// 99 normal values + 1 spike — p99 index = 98 → 100.0, spike at 99 ignored
		values := make([]float64, 99)
		for i := range values {
			values[i] = 100.0
		}
		values = append(values, 9999.0)
		result, err := ComputeP99(values)
		assert.NoError(t, err)
		assert.Equal(t, 100.0, result)
	})

	t.Run("original slice not modified", func(t *testing.T) {
		track(t)
		original := []float64{5.0, 3.0, 8.0, 1.0, 9.0}
		snapshot := make([]float64, len(original))
		copy(snapshot, original)
		ComputeP99(original)
		assert.Equal(t, snapshot, original)
	})

	t.Run("typical 7d workload — spike beyond p99 ignored", func(t *testing.T) {
		track(t)
		// 167 normal + 1 spike — p99 index = ceil(0.99×168)-1 = 166 → 120.0
		values := make([]float64, 167)
		for i := range values {
			values[i] = 120.0
		}
		values = append(values, 9999.0)
		result, err := ComputeP99(values)
		assert.NoError(t, err)
		assert.Equal(t, 120.0, result)
	})
}
