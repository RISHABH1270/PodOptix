package tests

import (
	"context"
	"net/http"
	"testing"

	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestRecommendations(t *testing.T) {
	tok := bearer(testToken())

	t.Run("GET /recommendations (cross-cluster)", func(t *testing.T) {
		t.Run("returns empty array when no recommendations exist", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/recommendations", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, byte('['), body[0]) // always array, never null
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			track(t)
			resp := do(t, http.MethodGet, "/api/v1/recommendations", "", "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})

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

	t.Run("DELETE /clusters/:id/recommendations/:recId", func(t *testing.T) {
		t.Run("deletes a single recommendation", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-one-cluster", "http://prom.del.one.test")
			rec := newRec(id, "ns", "Deployment", "svc", "api")
			assert.NoError(t, db.UpsertRecommendation(context.Background(), rec))

			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations/"+rec.RecommendationID, "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, `"deleted":1`)

			recs, _ := db.ListByCluster(context.Background(), id)
			assert.Empty(t, recs)
		})

		t.Run("unknown recId returns 404", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-missing-cluster", "http://prom.del.missing.test")
			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations/not-a-real-id", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			assert.Contains(t, body, "not found")
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-noauth-cluster", "http://prom.del.noauth.test")
			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations/anything", "", "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})

	t.Run("DELETE /clusters/:id/recommendations?orphaned=true", func(t *testing.T) {
		t.Run("bulk-deletes orphaned rows only", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-orph-cluster", "http://prom.del.orph.test")
			alive := newRec(id, "ns", "Deployment", "alive", "api")
			dead1 := newRec(id, "ns", "Deployment", "dead1", "api")
			dead2 := newRec(id, "ns", "Deployment", "dead2", "api")
			for _, r := range []*models.Recommendation{alive, dead1, dead2} {
				assert.NoError(t, db.UpsertRecommendation(context.Background(), r))
			}
			// mark everything except alive as orphaned
			_, err := db.MarkOrphaned(context.Background(), id, []models.WorkloadKey{
				{Namespace: "ns", WorkloadKind: "Deployment", WorkloadName: "alive", ContainerName: "api"},
			})
			assert.NoError(t, err)

			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations?orphaned=true", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, `"deleted":2`)

			recs, _ := db.ListByCluster(context.Background(), id)
			assert.Len(t, recs, 1)
			assert.Equal(t, "alive", recs[0].WorkloadName)
		})

		t.Run("without ?orphaned=true returns 400 (footgun guard)", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-guard-cluster", "http://prom.del.guard.test")
			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations", "", tok)
			body := readBody(t, resp)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, body, "?orphaned=true")
		})

		t.Run("no auth returns 401", func(t *testing.T) {
			track(t)
			id := createCluster(t, "del-orph-noauth-cluster", "http://prom.del.orph.noauth.test")
			resp := do(t, http.MethodDelete, "/api/v1/clusters/"+id+"/recommendations?orphaned=true", "", "")
			resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})
}
