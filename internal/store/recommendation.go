package store

import (
	"context"
	"fmt"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// UpsertRecommendation inserts a new recommendation or updates the existing one.
// One row per container — updated in place every time the scheduler runs.
func (s *Store) UpsertRecommendation(ctx context.Context, r *models.Recommendation) error {
	query := `
		INSERT INTO recommendations (
			recommendation_id, cluster_id, namespace, pod_name, container_name,
			status, current_cpu_limit, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_limit, recommended_mem_limit,
			lookback_window, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (cluster_id, namespace, pod_name, container_name)
		DO UPDATE SET
			status                = EXCLUDED.status,
			current_cpu_limit     = EXCLUDED.current_cpu_limit,
			current_mem_limit     = EXCLUDED.current_mem_limit,
			p99_cpu               = EXCLUDED.p99_cpu,
			p99_mem               = EXCLUDED.p99_mem,
			recommended_cpu_limit = EXCLUDED.recommended_cpu_limit,
			recommended_mem_limit = EXCLUDED.recommended_mem_limit,
			updated_at            = EXCLUDED.updated_at
	`
	_, err := s.pool.Exec(ctx, query,
		r.RecommendationID,
		r.ClusterID,
		r.Namespace,
		r.PodName,
		r.ContainerName,
		r.Status,
		r.CurrentCPULimit,
		r.CurrentMemLimit,
		r.P99CPU,
		r.P99Mem,
		r.RecommendedCPULimit,
		r.RecommendedMemLimit,
		r.LookbackWindow,
		r.CreatedAt,
		r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert recommendation: %w", err)
	}
	return nil
}

// ListByCluster fetches all recommendations for a given cluster.
func (s *Store) ListByCluster(ctx context.Context, clusterID string) ([]*models.Recommendation, error) {
	query := `
		SELECT
			recommendation_id, cluster_id, namespace, pod_name, container_name,
			status, current_cpu_limit, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_limit, recommended_mem_limit,
			lookback_window, created_at, updated_at
		FROM recommendations
		WHERE cluster_id = $1
		ORDER BY namespace, pod_name, container_name
	`
	rows, err := s.pool.Query(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list recommendations: %w", err)
	}
	defer rows.Close()

	var recommendations []*models.Recommendation
	for rows.Next() {
		r := &models.Recommendation{}
		err := rows.Scan(
			&r.RecommendationID,
			&r.ClusterID,
			&r.Namespace,
			&r.PodName,
			&r.ContainerName,
			&r.Status,
			&r.CurrentCPULimit,
			&r.CurrentMemLimit,
			&r.P99CPU,
			&r.P99Mem,
			&r.RecommendedCPULimit,
			&r.RecommendedMemLimit,
			&r.LookbackWindow,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan recommendation: %w", err)
		}
		recommendations = append(recommendations, r)
	}
	return recommendations, nil
}
