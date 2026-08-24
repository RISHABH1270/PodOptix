package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper — creates a cluster and returns its ID, fails the test if creation fails
func createTestCluster(t *testing.T, token, name, url string) string {
	t.Helper()
	body := `{
		"cluster_name":     "` + name + `",
		"prometheus_url":   "` + url + `",
		"prometheus_token": "test-token-123"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	id, ok := resp["cluster_id"].(string)
	if !ok {
		t.Fatalf("createTestCluster failed (status %d): %s", w.Code, w.Body.String())
	}
	return id
}

// ── POST /api/v1/clusters ──────────────────────────────────────────────────

func TestCreateCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{
		"cluster_name":     "test-cluster",
		"prometheus_url":   "http://prometheus.test.com",
		"prometheus_token": "test-token-123"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "test-cluster")
	assert.Contains(t, w.Body.String(), "http://prometheus.test.com")
	assert.Contains(t, w.Body.String(), "cluster_id")
	assert.Contains(t, w.Body.String(), "not yet synced")
}

func TestCreateCluster_MissingFields(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{"cluster_name": "incomplete-cluster"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestCreateCluster_InvalidLookback(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{
		"cluster_name":     "lookback-test",
		"prometheus_url":   "http://prometheus.test.com",
		"prometheus_token": "test-token-123",
		"lookback_window":  "99d"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "lookback_window")
}

func TestCreateCluster_NoAuth(t *testing.T) {
	trackTest(t)
	body := `{
		"cluster_name":     "no-auth-cluster",
		"prometheus_url":   "http://prometheus.test.com",
		"prometheus_token": "test-token-123"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── GET /api/v1/clusters ───────────────────────────────────────────────────

func TestListClusters(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// response must be an array, never null
	assert.True(t, w.Body.String()[0] == '[')
}

// ── GET /api/v1/clusters/:id ───────────────────────────────────────────────

func TestGetCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	id := createTestCluster(t, token, "get-test-cluster", "http://prometheus.get-test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "get-test-cluster")
}

func TestGetCluster_NotFound(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/non-existent-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Cluster not found")
}

// ── PUT /api/v1/clusters/:id ───────────────────────────────────────────────

func TestUpdateCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	id := createTestCluster(t, token, "update-test-cluster", "http://prometheus.update-test.com")

	body := `{"cluster_name": "updated-cluster-name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/clusters/"+id, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "updated-cluster-name")
}

func TestUpdateCluster_InvalidLookback(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	id := createTestCluster(t, token, "lookback-update-cluster", "http://prometheus.lookback-test.com")

	body := `{"lookback_window": "99d"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/clusters/"+id, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "lookback_window")
}

func TestUpdateCluster_NotFound(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{"cluster_name": "ghost"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/clusters/non-existent-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── DELETE /api/v1/clusters/:id ────────────────────────────────────────────

func TestDeleteCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	id := createTestCluster(t, token, "delete-test-cluster", "http://prometheus.delete-test.com")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// verify gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getW := httptest.NewRecorder()
	testServer.router.ServeHTTP(getW, getReq)
	assert.Equal(t, http.StatusNotFound, getW.Code)
}

func TestDeleteCluster_NotFound(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/non-existent-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
