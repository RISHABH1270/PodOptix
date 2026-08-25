package testkit

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createCluster is a helper that registers a cluster and returns its ID.
func createCluster(t *testing.T, name, url string) string {
	t.Helper()
	body := `{"cluster_name":"` + name + `","prometheus_url":"` + url + `","prometheus_token":"test-token-123"}`
	w := do(t, http.MethodPost, "/api/v1/clusters", body, testToken())
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	id, ok := res["cluster_id"].(string)
	if !ok {
		t.Fatalf("createCluster failed (status %d): %s", w.Code, w.Body.String())
	}
	return id
}

func TestClusters(t *testing.T) {
	tok := testToken()

	t.Run("POST /clusters", func(t *testing.T) {
		t.Run("success returns 201 with cluster_id and not yet synced", func(t *testing.T) {
			w := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"post-ok","prometheus_url":"http://prom.test.com","prometheus_token":"tok"}`, tok)
			assert.Equal(t, http.StatusCreated, w.Code)
			assert.Contains(t, w.Body.String(), "cluster_id")
			assert.Contains(t, w.Body.String(), "not yet synced")
		})

		t.Run("missing required fields returns 400", func(t *testing.T) {
			w := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"no-url"}`, tok)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("invalid lookback_window returns 400", func(t *testing.T) {
			w := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"bad-lb","prometheus_url":"http://p.test","prometheus_token":"tok","lookback_window":"99d"}`, tok)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "lookback_window")
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			w := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"no-auth","prometheus_url":"http://p.test","prometheus_token":"tok"}`, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})

	t.Run("GET /clusters", func(t *testing.T) {
		t.Run("returns 200 with array", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters", "", tok)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, byte('['), w.Body.Bytes()[0])
		})
	})

	t.Run("GET /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "get-test", "http://prom.get.test")

		t.Run("success returns cluster", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters/"+id, "", tok)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "get-test")
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters/non-existent-id", "", tok)
			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), "Cluster not found")
		})
	})

	t.Run("PUT /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "update-test", "http://prom.update.test")

		t.Run("success updates name", func(t *testing.T) {
			w := do(t, http.MethodPut, "/api/v1/clusters/"+id, `{"cluster_name":"updated-name"}`, tok)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "updated-name")
		})

		t.Run("invalid lookback_window returns 400", func(t *testing.T) {
			w := do(t, http.MethodPut, "/api/v1/clusters/"+id, `{"lookback_window":"99d"}`, tok)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "lookback_window")
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			w := do(t, http.MethodPut, "/api/v1/clusters/non-existent-id", `{"cluster_name":"ghost"}`, tok)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	})

	t.Run("DELETE /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "delete-test", "http://prom.delete.test")

		t.Run("success returns 204", func(t *testing.T) {
			w := do(t, http.MethodDelete, "/api/v1/clusters/"+id, "", tok)
			assert.Equal(t, http.StatusNoContent, w.Code)
		})

		t.Run("deleted cluster returns 404 on GET", func(t *testing.T) {
			w := do(t, http.MethodGet, "/api/v1/clusters/"+id, "", tok)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			w := do(t, http.MethodDelete, "/api/v1/clusters/non-existent-id", "", tok)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	})
}
