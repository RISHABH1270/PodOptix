package tests

import (
	"context"
	"testing"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// newRec builds a minimal ready recommendation for direct store insertion.
func newRec(clusterID, ns, kind, name, container string) *models.Recommendation {
	now := time.Now()
	return &models.Recommendation{
		RecommendationID: uuid.New().String(),
		ClusterID:        clusterID,
		Namespace:        ns,
		WorkloadKind:     kind,
		WorkloadName:     name,
		ContainerName:    container,
		ReplicaCount:     1,
		Status:           models.RecommendationStatusReady,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// seedCluster creates a cluster row directly so recommendations have a valid FK target.
// Isolated per-test cluster_id — no cleanup needed, test DB is dropped on exit.
func seedCluster(t *testing.T, clusterID string) {
	t.Helper()
	now := time.Now()
	err := db.SaveCluster(context.Background(), &models.Cluster{
		ClusterID:       clusterID,
		ClusterName:     "orphan-test-" + clusterID,
		PrometheusURL:   "http://p",
		PrometheusToken: "",
		LookbackWindow:  "7d",
		Status:          "connected",
		CreatedBy:       "test",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("seed cluster: %v", err)
	}
}

func TestOrphanLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("mark orphaned stamps rows not in seenKeys", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)

		recA := newRec(cid, "ns", "Deployment", "svc-a", "api")
		recB := newRec(cid, "ns", "Deployment", "svc-b", "api")
		recC := newRec(cid, "ns", "Deployment", "svc-c", "api")
		for _, r := range []*models.Recommendation{recA, recB, recC} {
			assert.NoError(t, db.UpsertRecommendation(ctx, r))
		}

		// Scheduler run observes only svc-a and svc-b. svc-c should be stamped.
		seen := []models.WorkloadKey{
			{Namespace: "ns", WorkloadKind: "Deployment", WorkloadName: "svc-a", ContainerName: "api"},
			{Namespace: "ns", WorkloadKind: "Deployment", WorkloadName: "svc-b", ContainerName: "api"},
		}
		n, err := db.MarkOrphaned(ctx, cid, seen)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)

		recs, _ := db.ListByCluster(ctx, cid)
		byName := map[string]*models.Recommendation{}
		for _, r := range recs {
			byName[r.WorkloadName] = r
		}
		assert.Nil(t, byName["svc-a"].OrphanedAt)
		assert.Nil(t, byName["svc-b"].OrphanedAt)
		assert.NotNil(t, byName["svc-c"].OrphanedAt)
	})

	t.Run("re-upserting an orphaned workload clears its tombstone", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)

		rec := newRec(cid, "ns", "Deployment", "comeback", "api")
		assert.NoError(t, db.UpsertRecommendation(ctx, rec))

		// Mark orphaned (empty seen set would be a no-op, so include a decoy workload)
		_, err := db.MarkOrphaned(ctx, cid, []models.WorkloadKey{
			{Namespace: "other", WorkloadKind: "Deployment", WorkloadName: "decoy", ContainerName: "x"},
		})
		assert.NoError(t, err)

		recs, _ := db.ListByCluster(ctx, cid)
		assert.NotNil(t, recs[0].OrphanedAt, "pre-condition: workload is tombstoned")

		// Workload returns on next scheduler run → upsert should clear orphaned_at.
		assert.NoError(t, db.UpsertRecommendation(ctx, rec))
		recs, _ = db.ListByCluster(ctx, cid)
		assert.Nil(t, recs[0].OrphanedAt, "upsert should clear orphaned_at — workload is alive again")
	})

	t.Run("safety gate — empty seenKeys is a no-op", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)
		assert.NoError(t, db.UpsertRecommendation(ctx, newRec(cid, "ns", "Deployment", "critical", "api")))

		// Prometheus hiccupped — scheduler saw nothing. Must NOT mark everything orphaned.
		n, err := db.MarkOrphaned(ctx, cid, nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, n)

		recs, _ := db.ListByCluster(ctx, cid)
		assert.Nil(t, recs[0].OrphanedAt, "existing workload must not be stamped on empty run")
	})

	t.Run("already-orphaned rows keep their original timestamp", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)
		assert.NoError(t, db.UpsertRecommendation(ctx, newRec(cid, "ns", "Deployment", "gone", "api")))

		// First mark
		_, err := db.MarkOrphaned(ctx, cid, []models.WorkloadKey{
			{Namespace: "x", WorkloadKind: "Deployment", WorkloadName: "y", ContainerName: "z"},
		})
		assert.NoError(t, err)
		recs, _ := db.ListByCluster(ctx, cid)
		firstStamp := *recs[0].OrphanedAt

		// Second mark some time later — timestamp must not advance
		time.Sleep(10 * time.Millisecond)
		n, err := db.MarkOrphaned(ctx, cid, []models.WorkloadKey{
			{Namespace: "x", WorkloadKind: "Deployment", WorkloadName: "y", ContainerName: "z"},
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, n, "already-orphaned rows should not be re-stamped")
		recs, _ = db.ListByCluster(ctx, cid)
		assert.Equal(t, firstStamp, *recs[0].OrphanedAt, "orphaned_at should preserve the time of first observation")
	})

	t.Run("DeleteOrphaned removes only orphaned rows", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)
		alive := newRec(cid, "ns", "Deployment", "alive", "api")
		dead1 := newRec(cid, "ns", "Deployment", "dead1", "api")
		dead2 := newRec(cid, "ns", "Deployment", "dead2", "api")
		for _, r := range []*models.Recommendation{alive, dead1, dead2} {
			assert.NoError(t, db.UpsertRecommendation(ctx, r))
		}
		_, err := db.MarkOrphaned(ctx, cid, []models.WorkloadKey{
			{Namespace: "ns", WorkloadKind: "Deployment", WorkloadName: "alive", ContainerName: "api"},
		})
		assert.NoError(t, err)

		n, err := db.DeleteOrphaned(ctx, cid)
		assert.NoError(t, err)
		assert.Equal(t, 2, n)

		recs, _ := db.ListByCluster(ctx, cid)
		assert.Len(t, recs, 1)
		assert.Equal(t, "alive", recs[0].WorkloadName)
	})

	t.Run("DeleteRecommendation removes one row by id", func(t *testing.T) {
		track(t)
		cid := uuid.New().String()
		seedCluster(t, cid)
		keep := newRec(cid, "ns", "Deployment", "keep", "api")
		drop := newRec(cid, "ns", "Deployment", "drop", "api")
		assert.NoError(t, db.UpsertRecommendation(ctx, keep))
		assert.NoError(t, db.UpsertRecommendation(ctx, drop))

		assert.NoError(t, db.DeleteRecommendation(ctx, drop.RecommendationID))
		recs, _ := db.ListByCluster(ctx, cid)
		assert.Len(t, recs, 1)
		assert.Equal(t, "keep", recs[0].WorkloadName)

		// Deleting a non-existent id returns an error
		err := db.DeleteRecommendation(ctx, uuid.New().String())
		assert.ErrorContains(t, err, "recommendation not found")
	})
}
