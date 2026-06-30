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

// Generate takes raw container metrics and produces a Recommendation.
// It computes p99 for CPU and memory, multiplies by 2, and returns
// a Recommendation ready to be upserted into the database.
func Generate(
	clusterID string,
	lookbackWindow string,
	currentCPULimit int,
	currentMemLimit int,
	metrics *collector.ContainerMetrics,
) (*models.Recommendation, error) {

	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	// compute p99 for CPU (millicores)
	p99CPU, err := compute.ComputeP99(metrics.CPUValues)
	if err != nil {
		return nil, fmt.Errorf("compute p99 cpu for %s/%s: %w",
			metrics.PodName, metrics.ContainerName, err)
	}

	// compute p99 for memory (MiB)
	p99Mem, err := compute.ComputeP99(metrics.MemValues)
	if err != nil {
		return nil, fmt.Errorf("compute p99 mem for %s/%s: %w",
			metrics.PodName, metrics.ContainerName, err)
	}

	// recommended limit = p99 × 2, rounded up to nearest integer
	recommendedCPU := int(math.Ceil(p99CPU * 2))
	recommendedMem := int(math.Ceil(p99Mem * 2))

	now := time.Now()

	return &models.Recommendation{
		RecommendationID:    uuid.New().String(),
		ClusterID:           clusterID,
		Namespace:           metrics.Namespace,
		PodName:             metrics.PodName,
		ContainerName:       metrics.ContainerName,
		Status:              models.StatusReady,
		CurrentCPULimit:     currentCPULimit,
		CurrentMemLimit:     currentMemLimit,
		P99CPU:              p99CPU,
		P99Mem:              p99Mem,
		RecommendedCPULimit: recommendedCPU,
		RecommendedMemLimit: recommendedMem,
		LookbackWindow:      lookbackWindow,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// GenerateAll generates recommendations for all containers in a cluster.
// Containers with insufficient data (empty CPU or memory values) are marked as new_service and skipped for p99 computation.
func GenerateAll(
	clusterID string,
	lookbackWindow string,
	allMetrics []*collector.ContainerMetrics,
) ([]*models.Recommendation, error) {

	var recommendations []*models.Recommendation

	for _, m := range allMetrics {
		// not enough data — mark as new_service
		if len(m.CPUValues) == 0 || len(m.MemValues) == 0 {
			now := time.Now()
			recommendations = append(recommendations, &models.Recommendation{
				RecommendationID: uuid.New().String(),
				ClusterID:        clusterID,
				Namespace:        m.Namespace,
				PodName:          m.PodName,
				ContainerName:    m.ContainerName,
				Status:           models.StatusNewService,
				LookbackWindow:   lookbackWindow,
				CreatedAt:        now,
				UpdatedAt:        now,
			})
			continue
		}

		rec, err := Generate(clusterID, lookbackWindow, 0, 0, m)
		if err != nil {
			return nil, fmt.Errorf("generate recommendation: %w", err)
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations, nil
}
