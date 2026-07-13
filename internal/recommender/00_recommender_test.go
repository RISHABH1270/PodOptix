package recommender

import (
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestGenerate_Success(t *testing.T) {
	metrics := &collector.ContainerMetrics{
		Namespace:     "payments",
		PodName:       "payment-api",
		ContainerName: "api",
		CPUValues:     []float64{100, 110, 120, 105, 115},
		MemValues:     []float64{200, 210, 220, 205, 215},
	}

	rec, err := Generate("cluster-123", "7d", 1000, 1024, metrics)

	assert.NoError(t, err)
	assert.Equal(t, "cluster-123", rec.ClusterID)
	assert.Equal(t, "payments", rec.Namespace)
	assert.Equal(t, "payment-api", rec.PodName)
	assert.Equal(t, "api", rec.ContainerName)
	assert.Equal(t, models.RecommendationStatusReady, rec.Status)
	assert.Equal(t, "7d", rec.LookbackWindow)
	assert.Equal(t, 1000, rec.CurrentCPULimit)
	assert.Equal(t, 1024, rec.CurrentMemLimit)
	// p99 of [100,105,110,115,120] = 120, recommended = ceil(120*2) = 240
	assert.Equal(t, 240, rec.RecommendedCPULimit)
	// p99 of [200,205,210,215,220] = 220, recommended = ceil(220*2) = 440
	assert.Equal(t, 440, rec.RecommendedMemLimit)
	assert.NotEmpty(t, rec.RecommendationID)
}

func TestGenerate_NilMetrics(t *testing.T) {
	_, err := Generate("cluster-123", "7d", 0, 0, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metrics cannot be nil")
}

func TestGenerate_EmptyCPU(t *testing.T) {
	metrics := &collector.ContainerMetrics{
		Namespace:     "payments",
		PodName:       "payment-api",
		ContainerName: "api",
		CPUValues:     []float64{}, // empty
		MemValues:     []float64{200, 210},
	}
	_, err := Generate("cluster-123", "7d", 0, 0, metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compute p99 cpu")
}

func TestGenerate_EmptyMem(t *testing.T) {
	metrics := &collector.ContainerMetrics{
		Namespace:     "payments",
		PodName:       "payment-api",
		ContainerName: "api",
		CPUValues:     []float64{100, 110},
		MemValues:     []float64{}, // empty
	}
	_, err := Generate("cluster-123", "7d", 0, 0, metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compute p99 mem")
}

func TestGenerate_RecommendationIsDoubleP99(t *testing.T) {
	metrics := &collector.ContainerMetrics{
		Namespace:     "ns",
		PodName:       "pod",
		ContainerName: "c",
		CPUValues:     []float64{50},
		MemValues:     []float64{100},
	}

	rec, err := Generate("cluster-1", "7d", 0, 0, metrics)
	assert.NoError(t, err)
	// p99 of single value = 50, recommended = 50*2 = 100
	assert.Equal(t, 100, rec.RecommendedCPULimit)
	// p99 of single value = 100, recommended = 100*2 = 200
	assert.Equal(t, 200, rec.RecommendedMemLimit)
}

func TestGenerateAll_MixedData(t *testing.T) {
	allMetrics := []*collector.ContainerMetrics{
		{
			Namespace: "ns", PodName: "pod-1", ContainerName: "c1",
			CPUValues: []float64{100, 110, 120},
			MemValues: []float64{200, 210, 220},
		},
		{
			Namespace: "ns", PodName: "pod-2", ContainerName: "c2",
			CPUValues: []float64{}, // new service — no data
			MemValues: []float64{},
		},
	}

	recs, err := GenerateAll("cluster-1", "7d", allMetrics)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(recs))
	assert.Equal(t, models.RecommendationStatusReady, recs[0].Status)
	assert.Equal(t, models.RecommendationStatusNewService, recs[1].Status)
}

func TestGenerateAll_Empty(t *testing.T) {
	recs, err := GenerateAll("cluster-1", "7d", []*collector.ContainerMetrics{})
	assert.NoError(t, err)
	assert.Empty(t, recs)
}
