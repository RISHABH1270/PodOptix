package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── parseDuration tests ────────────────────────────────────────────────────

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	assert.NoError(t, err)
	assert.Equal(t, 7*24*60*60, int(d.Seconds()))
}

func TestParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("24h")
	assert.NoError(t, err)
	assert.Equal(t, 24*60*60, int(d.Seconds()))
}

func TestParseDuration_Minutes(t *testing.T) {
	d, err := parseDuration("30m")
	assert.NoError(t, err)
	assert.Equal(t, 30*60, int(d.Seconds()))
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("xyz")
	assert.Error(t, err)
}

func TestParseDuration_UnknownUnit(t *testing.T) {
	_, err := parseDuration("7w")
	assert.Error(t, err)
}

// ─── extractValues tests ────────────────────────────────────────────────────

func TestExtractValues_Valid(t *testing.T) {
	values := [][]interface{}{
		{1719100800, "120.5"},
		{1719104400, "115.2"},
		{1719108000, "132.8"},
	}
	result := extractValues(values)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 120.5, result[0])
	assert.Equal(t, 115.2, result[1])
	assert.Equal(t, 132.8, result[2])
}

func TestExtractValues_Empty(t *testing.T) {
	result := extractValues([][]interface{}{})
	assert.Empty(t, result)
}

func TestExtractValues_InvalidValue(t *testing.T) {
	values := [][]interface{}{
		{1719100800, "notanumber"},
		{1719104400, "120.5"},
	}
	result := extractValues(values)
	// invalid value skipped, valid one kept
	assert.Equal(t, 1, len(result))
	assert.Equal(t, 120.5, result[0])
}

// ─── Collect tests using fake Prometheus HTTP server ────────────────────────

// fakePrometheusResponse builds a Prometheus /api/v1/query_range response
// with one container result and given values.
func fakePrometheusResponse(namespace, pod, container string, values [][]interface{}) string {
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "matrix",
			"result": []map[string]interface{}{
				{
					"metric": map[string]string{
						"namespace": namespace,
						"pod":       pod,
						"container": container,
					},
					"values": values,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestCollect_Success(t *testing.T) {
	cpuValues := [][]interface{}{{1719100800, "120.5"}, {1719104400, "115.2"}}
	memValues := [][]interface{}{{1719100800, "180.2"}, {1719104400, "178.9"}}

	callCount := 0
	// fake Prometheus server — returns CPU response first, Memory response second
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// first call = CPU query
			w.Write([]byte(fakePrometheusResponse("payments", "payment-api", "api", cpuValues)))
		} else {
			// second call = Memory query
			w.Write([]byte(fakePrometheusResponse("payments", "payment-api", "api", memValues)))
		}
	}))
	defer server.Close()

	c := New(server.URL, "")
	metrics, err := c.Collect(context.Background(), "7d")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(metrics))
	assert.Equal(t, "payments", metrics[0].Namespace)
	assert.Equal(t, "payment-api", metrics[0].PodName)
	assert.Equal(t, "api", metrics[0].ContainerName)
	assert.Equal(t, 2, len(metrics[0].CPUValues))
	assert.Equal(t, 2, len(metrics[0].MemValues))
	assert.Equal(t, 120.5, metrics[0].CPUValues[0])
	assert.Equal(t, 180.2, metrics[0].MemValues[0])
}

func TestCollect_PrometheusError(t *testing.T) {
	// fake server that always returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(server.URL, "")
	_, err := c.Collect(context.Background(), "7d")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prometheus returned status 500")
}

func TestCollect_EmptyResponse(t *testing.T) {
	// fake server that returns success but no containers
	emptyResp := `{"status":"success","data":{"resultType":"matrix","result":[]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(emptyResp))
	}))
	defer server.Close()

	c := New(server.URL, "")
	metrics, err := c.Collect(context.Background(), "7d")

	assert.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestCollect_WithToken(t *testing.T) {
	receivedToken := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("Authorization")
		emptyResp := `{"status":"success","data":{"resultType":"matrix","result":[]}}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(emptyResp))
	}))
	defer server.Close()

	c := New(server.URL, "my-secret-token")
	c.Collect(context.Background(), "7d")

	assert.Equal(t, "Bearer my-secret-token", receivedToken)
}

func TestCollect_InvalidDuration(t *testing.T) {
	c := New("http://localhost:9090", "")
	_, err := c.Collect(context.Background(), "invalid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse lookback window")
}
