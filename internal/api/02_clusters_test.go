package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateCluster tests POST /api/v1/clusters
func TestCreateCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{
		"name":           "test-cluster",
		"prometheus_url": "http://prometheus.test.com",
		"token":          "test-token-123"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "test-cluster")
}

// TestCreateCluster_MissingFields tests POST /api/v1/clusters with missing fields
func TestCreateCluster_MissingFields(t *testing.T) {
	trackTest(t)
	token := getTestToken()
	body := `{"name": "incomplete-cluster"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListClusters tests GET /api/v1/clusters
func TestListClusters(t *testing.T) {
	trackTest(t)
	token := getTestToken()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetCluster tests GET /api/v1/clusters/:id
func TestGetCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()

	// create a cluster
	body := `{
		"name":           "get-test-cluster",
		"prometheus_url": "http://prometheus.get-test.com",
		"token":          "get-test-token"
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	testServer.router.ServeHTTP(createW, createReq)

	var created map[string]any
	json.Unmarshal(createW.Body.Bytes(), &created)
	id, ok := created["cluster_id"].(string)
	if !ok {
		t.Fatalf("create cluster failed: %s", createW.Body.String())
	}

	// fetch by id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testServer.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "get-test-cluster")
}

// TestGetCluster_NotFound tests GET /api/v1/clusters/:id with non-existent id
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

// TestDeleteCluster tests DELETE /api/v1/clusters/:id
func TestDeleteCluster(t *testing.T) {
	trackTest(t)
	token := getTestToken()

	// create cluster to delete
	body := `{
		"name":           "delete-test-cluster",
		"prometheus_url": "http://prometheus.delete-test.com",
		"token":          "delete-test-token"
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	testServer.router.ServeHTTP(createW, createReq)

	var created map[string]any
	json.Unmarshal(createW.Body.Bytes(), &created)
	id, ok := created["cluster_id"].(string)
	if !ok {
		t.Fatalf("create cluster failed: %s", createW.Body.String())
	}

	// delete it
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
