package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecommendations(t *testing.T) {
	tok := bearer(testToken())

	t.Run("GET /clusters/:id/recommendations", func(t *testing.T) {
		t.Run("returns empty array for new cluster", func(t *testing.T) {
			track(t)
			id := createCluster(t, "rec-list-cluster", "http://prom.rec.test")
			resp := do(t, http.MethodGet, "/api/v1/clusters/"+id+"/recommendations", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, byte('['), body[0]) // always array, never null
		})

		t.Run("unknown cluster id returns empty array", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/clusters/non-existent-id/recommendations", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, byte('['), body[0])
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			track(t)
			id := createCluster(t, "rec-noauth-cluster", "http://prom.rec.noauth.test")
			resp := do(t, http.MethodGet, "/api/v1/clusters/"+id+"/recommendations", "", "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})

	t.Run("POST /clusters/:id/recalculate", func(t *testing.T) {
		t.Run("returns 202 accepted immediately", func(t *testing.T) {
			track(t)
			id := createCluster(t, "recalc-cluster", "http://prom.recalc.test")
			resp := do(t, http.MethodPost, "/api/v1/clusters/"+id+"/recalculate", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
			assert.Contains(t, body, "cluster_id")
			assert.Contains(t, body, "Recalculation started")
		})

		t.Run("duplicate recalculate returns 429", func(t *testing.T) {
			track(t)
			id := createCluster(t, "recalc-dup-cluster", "http://prom.recalc.dup.test")
			// first call acquires lock
			do(t, http.MethodPost, "/api/v1/clusters/"+id+"/recalculate", "", tok).Body.Close()
			// second call should be rejected
			resp := do(t, http.MethodPost, "/api/v1/clusters/"+id+"/recalculate", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
			assert.Contains(t, body, "already in progress")
		})

		t.Run("unknown cluster id returns 404", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodPost, "/api/v1/clusters/non-existent-id/recalculate", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.Contains(t, body, "Cluster not found")
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			track(t)
			id := createCluster(t, "recalc-noauth-cluster", "http://prom.recalc.noauth.test")
			resp := do(t, http.MethodPost, "/api/v1/clusters/"+id+"/recalculate", "", "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})
}
