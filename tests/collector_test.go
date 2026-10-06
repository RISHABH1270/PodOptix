package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/stretchr/testify/assert"
)

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

var emptyCollectorResp = `{"status":"success","data":{"resultType":"matrix","result":[]}}`

func TestParseDuration(t *testing.T) {
	t.Run("days", func(t *testing.T) {
		track(t)
		d, err := collector.ParseDuration("7d")
		assert.NoError(t, err)
		assert.Equal(t, 7*24*60*60, int(d.Seconds()))
	})
	t.Run("hours", func(t *testing.T) {
		track(t)
		d, err := collector.ParseDuration("24h")
		assert.NoError(t, err)
		assert.Equal(t, 24*60*60, int(d.Seconds()))
	})
	t.Run("minutes", func(t *testing.T) {
		track(t)
		d, err := collector.ParseDuration("30m")
		assert.NoError(t, err)
		assert.Equal(t, 30*60, int(d.Seconds()))
	})
	t.Run("invalid value returns error", func(t *testing.T) {
		track(t)
		_, err := collector.ParseDuration("xyz")
		assert.Error(t, err)
	})
	t.Run("unknown unit returns error", func(t *testing.T) {
		track(t)
		_, err := collector.ParseDuration("7w")
		assert.Error(t, err)
	})
}

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
		metrics, err := collector.New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Len(t, metrics, 1)
		assert.Equal(t, "payments", metrics[0].Namespace)
		assert.Equal(t, []float64{120.5, 115.2}, metrics[0].CPUValues)
		assert.Equal(t, []float64{180.2, 178.9}, metrics[0].MemValues)
	})

	t.Run("prometheus 500 returns error", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		_, err := collector.New(srv.URL, "").Collect(context.Background(), "7d")
		assert.ErrorContains(t, err, "prometheus returned status 500")
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyCollectorResp))
		}))
		defer srv.Close()
		metrics, err := collector.New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Empty(t, metrics)
	})

	t.Run("token sent as Bearer header", func(t *testing.T) {
		track(t)
		var received string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyCollectorResp))
		}))
		defer srv.Close()
		collector.New(srv.URL, "my-secret-token").Collect(context.Background(), "7d")
		assert.Equal(t, "Bearer my-secret-token", received)
	})

	t.Run("invalid duration returns error", func(t *testing.T) {
		track(t)
		_, err := collector.New("http://localhost:9090", "").Collect(context.Background(), "invalid")
		assert.ErrorContains(t, err, "parse lookback window")
	})

	// Two replicas of the same Deployment collapse into one workload row.
	// At each shared timestamp the output takes MAX across replicas.
	t.Run("multi-replica collapse — pods owned by same Deployment yield one row with MAX aggregation", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			q := r.URL.Query().Get("query")

			// Matrix response helper for range queries
			matrix := func(series []map[string]interface{}) string {
				resp := map[string]interface{}{
					"status": "success",
					"data":   map[string]interface{}{"resultType": "matrix", "result": series},
				}
				b, _ := json.Marshal(resp)
				return string(b)
			}
			// Vector response helper for instant queries
			vector := func(series []map[string]interface{}) string {
				resp := map[string]interface{}{
					"status": "success",
					"data":   map[string]interface{}{"resultType": "vector", "result": series},
				}
				b, _ := json.Marshal(resp)
				return string(b)
			}

			switch {
			case strings.HasPrefix(q, "rate(container_cpu_usage_seconds_total"):
				// pod-a: 100 @ t1, 110 @ t2 · pod-b: 150 @ t1, 90 @ t2 → MAX = 150, 110
				w.Write([]byte(matrix([]map[string]interface{}{
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-abc", "container": "api"},
						"values": [][]interface{}{{1719100800, "100"}, {1719104400, "110"}}},
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-def", "container": "api"},
						"values": [][]interface{}{{1719100800, "150"}, {1719104400, "90"}}},
				})))
			case strings.HasPrefix(q, "container_memory_working_set_bytes"):
				w.Write([]byte(matrix([]map[string]interface{}{
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-abc", "container": "api"},
						"values": [][]interface{}{{1719100800, "200"}, {1719104400, "180"}}},
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-def", "container": "api"},
						"values": [][]interface{}{{1719100800, "190"}, {1719104400, "220"}}},
				})))
			case q == "kube_pod_owner":
				// Both pods owned by ReplicaSet auth-7f (common Deployment-generated name)
				w.Write([]byte(vector([]map[string]interface{}{
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-abc", "owner_kind": "ReplicaSet", "owner_name": "auth-7f"},
						"value": []interface{}{1719100800, "1"}},
					{"metric": map[string]string{"namespace": "ns", "pod": "auth-def", "owner_kind": "ReplicaSet", "owner_name": "auth-7f"},
						"value": []interface{}{1719100800, "1"}},
				})))
			case strings.HasPrefix(q, "kube_replicaset_owner"):
				// ReplicaSet auth-7f owned by Deployment auth-service
				w.Write([]byte(vector([]map[string]interface{}{
					{"metric": map[string]string{"namespace": "ns", "replicaset": "auth-7f", "owner_kind": "Deployment", "owner_name": "auth-service"},
						"value": []interface{}{1719100800, "1"}},
				})))
			default:
				// requests/limits — return empty for simplicity
				w.Write([]byte(emptyCollectorResp))
			}
		}))
		defer srv.Close()

		metrics, err := collector.New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Len(t, metrics, 1, "two pods should collapse into one workload row")
		m := metrics[0]
		assert.Equal(t, "ns", m.Namespace)
		assert.Equal(t, "Deployment", m.WorkloadKind)
		assert.Equal(t, "auth-service", m.WorkloadName)
		assert.Equal(t, "api", m.ContainerName)
		assert.Equal(t, 2, m.ReplicaCount)
		assert.Equal(t, []float64{150, 110}, m.CPUValues, "MAX across replicas per timestamp")
		assert.Equal(t, []float64{200, 220}, m.MemValues, "MAX across replicas per timestamp")
	})

	// kube_pod_owner missing → fall back to Pod/pod-name (Phase 1 behavior)
	t.Run("owner lookup unavailable — falls back to bare Pod naming", func(t *testing.T) {
		track(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			q := r.URL.Query().Get("query")
			switch {
			case strings.HasPrefix(q, "rate(container_cpu_usage_seconds_total"),
				strings.HasPrefix(q, "container_memory_working_set_bytes"):
				w.Write([]byte(fakeRangeResponse("ns", "bare-pod", "api", [][]interface{}{{1719100800, "100"}})))
			default:
				// Everything else (including kube_pod_owner) returns empty
				w.Write([]byte(emptyCollectorResp))
			}
		}))
		defer srv.Close()

		metrics, err := collector.New(srv.URL, "").Collect(context.Background(), "7d")
		assert.NoError(t, err)
		assert.Len(t, metrics, 1)
		assert.Equal(t, "Pod", metrics[0].WorkloadKind)
		assert.Equal(t, "bare-pod", metrics[0].WorkloadName)
		assert.Equal(t, 1, metrics[0].ReplicaCount)
	})
}
