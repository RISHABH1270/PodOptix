package tests

import (
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	t.Run("success — recommended is ceil(p99 × 2)", func(t *testing.T) {
		track(t)
		metrics := &collector.ContainerMetrics{
			Namespace: "payments", PodName: "payment-api", ContainerName: "api",
			CPUValues: []float64{100, 110, 120, 105, 115},
			MemValues: []float64{200, 210, 220, 205, 215},
			CPULimit:  1000, MemLimit: 1024,
		}
		rec, err := recommender.Generate("cluster-123", metrics)
		assert.NoError(t, err)
		assert.Equal(t, "cluster-123", rec.ClusterID)
		assert.Equal(t, models.RecommendationStatusReady, rec.Status)
		assert.Equal(t, 1000, rec.CurrentCPULimit)
		assert.Equal(t, 1024, rec.CurrentMemLimit)
		assert.Equal(t, 240, rec.RecommendedCPULimit)
		assert.Equal(t, 440, rec.RecommendedMemLimit)
		assert.NotEmpty(t, rec.RecommendationID)
	})

	t.Run("nil metrics returns error", func(t *testing.T) {
		track(t)
		_, err := recommender.Generate("cluster-123", nil)
		assert.ErrorContains(t, err, "metrics cannot be nil")
	})

	t.Run("empty CPU values returns error", func(t *testing.T) {
		track(t)
		_, err := recommender.Generate("cluster-123", &collector.ContainerMetrics{
			CPUValues: []float64{}, MemValues: []float64{200, 210},
		})
		assert.ErrorContains(t, err, "compute p99 cpu")
	})

	t.Run("empty memory values returns error", func(t *testing.T) {
		track(t)
		_, err := recommender.Generate("cluster-123", &collector.ContainerMetrics{
			CPUValues: []float64{100, 110}, MemValues: []float64{},
		})
		assert.ErrorContains(t, err, "compute p99 mem")
	})

	t.Run("single value — recommended is double", func(t *testing.T) {
		track(t)
		rec, err := recommender.Generate("cluster-1", &collector.ContainerMetrics{
			Namespace: "ns", PodName: "pod", ContainerName: "c",
			CPUValues: []float64{50}, MemValues: []float64{100},
		})
		assert.NoError(t, err)
		assert.Equal(t, 100, rec.RecommendedCPULimit)
		assert.Equal(t, 200, rec.RecommendedMemLimit)
	})
}

func TestGenerateAll(t *testing.T) {
	t.Run("mixed data — ready and new_service", func(t *testing.T) {
		track(t)
		recs, err := recommender.GenerateAll("cluster-1", []*collector.ContainerMetrics{
			{Namespace: "ns", PodName: "pod-1", ContainerName: "c1", CPUValues: []float64{100, 110, 120}, MemValues: []float64{200, 210, 220}},
			{Namespace: "ns", PodName: "pod-2", ContainerName: "c2", CPUValues: []float64{}, MemValues: []float64{}},
		})
		assert.NoError(t, err)
		assert.Len(t, recs, 2)
		assert.Equal(t, models.RecommendationStatusReady, recs[0].Status)
		assert.Equal(t, models.RecommendationStatusNewService, recs[1].Status)
	})

	t.Run("empty metrics returns empty slice", func(t *testing.T) {
		track(t)
		recs, err := recommender.GenerateAll("cluster-1", []*collector.ContainerMetrics{})
		assert.NoError(t, err)
		assert.Empty(t, recs)
	})
}
