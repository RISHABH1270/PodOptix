package testkit

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createCluster registers a cluster and returns its ID.
func createCluster(t *testing.T, name, url string) string {
	t.Helper()
	body := `{"cluster_name":"` + name + `","prometheus_url":"` + url + `","prometheus_token":"test-token-123"}`
	resp := do(t, http.MethodPost, "/api/v1/clusters", body, testToken())
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var res map[string]any
	json.Unmarshal(b, &res)
	id, ok := res["cluster_id"].(string)
	if !ok {
		t.Fatalf("createCluster failed (status %d): %s", resp.StatusCode, string(b))
	}
	return id
}

func TestClusters(t *testing.T) {
	tok := testToken()

	t.Run("POST /clusters", func(t *testing.T) {
		t.Run("success returns 201 with cluster_id and not yet synced", func(t *testing.T) {
			resp := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"post-ok","prometheus_url":"http://prom.test.com","prometheus_token":"tok"}`, tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
			assert.Contains(t, body, "cluster_id")
			assert.Contains(t, body, "not yet synced")
		})

		t.Run("missing required fields returns 400", func(t *testing.T) {
			resp := do(t, http.MethodPost, "/api/v1/clusters", `{"cluster_name":"no-url"}`, tok)
			resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("invalid lookback_window returns 400", func(t *testing.T) {
			resp := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"bad-lb","prometheus_url":"http://p.test","prometheus_token":"tok","lookback_window":"99d"}`, tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, "lookback_window")
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			resp := do(t, http.MethodPost, "/api/v1/clusters",
				`{"cluster_name":"no-auth","prometheus_url":"http://p.test","prometheus_token":"tok"}`, "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})

	t.Run("GET /clusters", func(t *testing.T) {
		t.Run("returns 200 with array", func(t *testing.T) {
			resp := do(t, http.MethodGet, "/api/v1/clusters", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, byte('['), body[0])
		})
	})

	t.Run("GET /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "get-test", "http://prom.get.test")

		t.Run("success returns cluster", func(t *testing.T) {
			resp := do(t, http.MethodGet, "/api/v1/clusters/"+id, "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "get-test")
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			resp := do(t, http.MethodGet, "/api/v1/clusters/non-existent-id", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.Contains(t, body, "Cluster not found")
		})
	})

	t.Run("PUT /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "update-test", "http://prom.update.test")

		t.Run("success updates name", func(t *testing.T) {
			resp := do(t, http.MethodPut, "/api/v1/clusters/"+id, `{"cluster_name":"updated-name"}`, tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "updated-name")
		})

		t.Run("invalid lookback_window returns 400", func(t *testing.T) {
			resp := do(t, http.MethodPut, "/api/v1/clusters/"+id, `{"lookback_window":"99d"}`, tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, "lookback_window")
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			resp := do(t, http.MethodPut, "/api/v1/clusters/non-existent-id", `{"cluster_name":"ghost"}`, tok)
			resp.Body.Close()
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	})

	t.Run("DELETE /clusters/:id", func(t *testing.T) {
		id := createCluster(t, "delete-test", "http://prom.delete.test")

		t.Run("success returns 204", func(t *testing.T) {
			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id, "", tok)
			resp.Body.Close()
			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		})

		t.Run("deleted cluster returns 404 on GET", func(t *testing.T) {
			resp := do(t, http.MethodGet, "/api/v1/clusters/"+id, "", tok)
			resp.Body.Close()
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("unknown id returns 404", func(t *testing.T) {
			resp := do(t, http.MethodDelete, "/api/v1/clusters/non-existent-id", "", tok)
			resp.Body.Close()
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	})
}
