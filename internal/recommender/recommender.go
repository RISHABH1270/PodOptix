package recommender

import (
	"fmt"
	"math"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/compute"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/google/uuid"
)

// generate computes p99 CPU/memory from usage history and pairs it with the container's
// current resource limits (from kube-state-metrics via ContainerMetrics).
func generate(clusterID string, metrics *collector.ContainerMetrics) (*models.Recommendation, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	p99CPU, err := compute.ComputeP99(metrics.CPUValues)
	if err != nil {
		return nil, fmt.Errorf("compute p99 cpu for %s/%s: %w", metrics.PodName, metrics.ContainerName, err)
	}

	p99Mem, err := compute.ComputeP99(metrics.MemValues)
	if err != nil {
		return nil, fmt.Errorf("compute p99 mem for %s/%s: %w", metrics.PodName, metrics.ContainerName, err)
	}

	now := time.Now()

	return &models.Recommendation{
		RecommendationID:    uuid.New().String(),
		ClusterID:           clusterID,
		Namespace:           metrics.Namespace,
		PodName:             metrics.PodName,
		ContainerName:       metrics.ContainerName,
		Status:              models.RecommendationStatusReady,
		CurrentCPULimit:     metrics.CPULimit,
		CurrentMemLimit:     metrics.MemLimit,
		P99CPU:              p99CPU,
		P99Mem:              p99Mem,
		RecommendedCPULimit: int(math.Ceil(p99CPU * 2)),
		RecommendedMemLimit: int(math.Ceil(p99Mem * 2)),
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// GenerateAll generates recommendations for all containers in a cluster.
// Containers with insufficient data are marked as new_service — check back after the cluster's lookback window.
func GenerateAll(clusterID string, allMetrics []*collector.ContainerMetrics) ([]*models.Recommendation, error) {
	var recommendations []*models.Recommendation

	for _, m := range allMetrics {
		if len(m.CPUValues) == 0 || len(m.MemValues) == 0 {
			now := time.Now()
			recommendations = append(recommendations, &models.Recommendation{
				RecommendationID: uuid.New().String(),
				ClusterID:        clusterID,
				Namespace:        m.Namespace,
				PodName:          m.PodName,
				ContainerName:    m.ContainerName,
				Status:           models.RecommendationStatusNewService,
				CreatedAt:        now,
				UpdatedAt:        now,
			})
			continue
		}

		rec, err := generate(clusterID, m)
		if err != nil {
			return nil, fmt.Errorf("generate recommendation: %w", err)
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations, nil
}
