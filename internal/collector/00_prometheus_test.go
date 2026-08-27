package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── test counter ─────────────────────────────────────────────────────────────

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
	log("\n  Running Collector Tests...\n")
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

// ── helpers ───────────────────────────────────────────────────────────────────

func fakeRangeResponse(namespace, pod, container string, values [][]interface{}) string {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "matrix",
			"result": []map[string]interface{}{
				{
					"metric": map[string]string{"namespace": namespace, "pod": pod, "container": container},
					"values": values,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

var emptyResp = `{"status":"success","data":{"resultType":"matrix","result":[]}}`

// ── parseDuration ─────────────────────────────────────────────────────────────

func TestParseDuration(t *testing.T) {
	t.Run("days", func(t *testing.T) {
		track(t)
		d, err := parseDuration("7d")
		assert.NoError(t, err)
		assert.Equal(t, 7*24*60*60, int(d.Seconds()))
	})
	t.Run("hours", func(t *testing.T) {
		track(t)
		d, err := parseDuration("24h")
		assert.NoError(t, err)
		assert.Equal(t, 24*60*60, int(d.Seconds()))
	})
	t.Run("minutes", func(t *testing.T) {
		track(t)
		d, err := parseDuration("30m")
		assert.NoError(t, err)
		assert.Equal(t, 30*60, int(d.Seconds()))
	})
	t.Run("invalid value returns error", func(t *testing.T) {
		track(t)
		_, err := parseDuration("xyz")
		assert.Error(t, err)
	})
	t.Run("unknown unit returns error", func(t *testing.T) {
		track(t)
		_, err := parseDuration("7w")
		assert.Error(t, err)
	})
}

// ── extractValues ─────────────────────────────────────────────────────────────

func TestExtractValues(t *testing.T) {
	t.Run("valid values parsed correctly", func(t *testing.T) {
		track(t)
		result := extractValues([][]interface{}{
			{1719100800, "120.5"},
			{1719104400, "115.2"},
			{1719108000, "132.8"},
		})
		assert.Equal(t, []float64{120.5, 115.2, 132.8}, result)
	})
	t.Run("empty input returns nil", func(t *testing.T) {
		track(t)
		assert.Empty(t, extractValues([][]interface{}{}))
	})
	t.Run("invalid value skipped, valid one kept", func(t *testing.T) {
		track(t)
		result := extractValues([][]interface{}{
			{1719100800, "notanumber"},
			{1719104400, "120.5"},
		})
		assert.Equal(t, []float64{120.5}, result)
	})
}

// ── Collect ───────────────────────────────────────────────────────────────────

func TestCollect(t *testing.T) {
	t.Run("success returns merged cpu and memory per container", func(t *testing.T) {
		track(t)
		cpuValues := [][]interface{}{{1719100800, "120.5"}, {1719104400, "115.2"}}
		memValues := [][]interface{}{{1719100800, "180.2"}, {1719104400, "178.9"}}
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			callCount++
			if callCount == 1 {
				w.Write([]byte(fakeRangeResponse("payments", "payment-api", "api", cpuValues)))
			} else {
				w.Write([]byte(fakeRangeResponse("payments", "payment-api", "api", memValues)))
			}
		}))
		defer srv.Close()

		metrics, err := New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Len(t, metrics, 1)
		assert.Equal(t, "payments", metrics[0].Namespace)
		assert.Equal(t, "payment-api", metrics[0].PodName)
		assert.Equal(t, "api", metrics[0].ContainerName)
		assert.Equal(t, []float64{120.5, 115.2}, metrics[0].CPUValues)
		assert.Equal(t, []float64{180.2, 178.9}, metrics[0].MemValues)
	})

	t.Run("prometheus 500 returns error", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := New(srv.URL, "").Collect(context.Background(), "7d")
		assert.ErrorContains(t, err, "prometheus returned status 500")
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyResp))
		}))
		defer srv.Close()

		metrics, err := New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Empty(t, metrics)
	})

	t.Run("token sent as Bearer header", func(t *testing.T) {
		track(t)
		var received string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyResp))
		}))
		defer srv.Close()

		New(srv.URL, "my-secret-token").Collect(context.Background(), "7d")
		assert.Equal(t, "Bearer my-secret-token", received)
	})

	t.Run("invalid duration returns error", func(t *testing.T) {
		track(t)
		_, err := New("http://localhost:9090", "").Collect(context.Background(), "invalid")
		assert.ErrorContains(t, err, "parse lookback window")
	})
}
