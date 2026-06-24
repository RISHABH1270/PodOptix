package store

import (
	"context"
	"fmt"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// SaveRecommendation inserts a new recommendation into the database.
func (s *Store) SaveRecommendation(ctx context.Context, r *models.Recommendation) error {
	query := `
		INSERT INTO recommendations (
			id, cluster_id, namespace, pod_name, container_name,
			current_cpu_limit, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_limit, recommended_mem_limit,
			lookback_window, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := s.pool.Exec(ctx, query,
		r.ID,
		r.ClusterID,
		r.Namespace,
		r.PodName,
		r.ContainerName,
		r.CurrentCPULimit,
		r.CurrentMemLimit,
		r.P99CPU,
		r.P99Mem,
		r.RecommendedCPULimit,
		r.RecommendedMemLimit,
		r.LookbackWindow,
		r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save recommendation: %w", err)
	}
	return nil
}

// ListByCluster fetches all recommendations for a given cluster.
func (s *Store) ListByCluster(ctx context.Context, clusterID string) ([]*models.Recommendation, error) {
	query := `
		SELECT
			id, cluster_id, namespace, pod_name, container_name,
			current_cpu_limit, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_limit, recommended_mem_limit,
			lookback_window, created_at
		FROM recommendations
		WHERE cluster_id = $1
		ORDER BY created_at DESC
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
			&r.ID,
			&r.ClusterID,
			&r.Namespace,
			&r.PodName,
			&r.ContainerName,
			&r.CurrentCPULimit,
			&r.CurrentMemLimit,
			&r.P99CPU,
			&r.P99Mem,
			&r.RecommendedCPULimit,
			&r.RecommendedMemLimit,
			&r.LookbackWindow,
			&r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan recommendation: %w", err)
		}
		recommendations = append(recommendations, r)
	}
	return recommendations, nil
}
