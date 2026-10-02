package store

import (
	"context"
	"fmt"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// ── Create / Update (Upsert) ──────────────────────────────────────────────────

// UpsertRecommendation inserts a new recommendation or updates the existing one.
// One row per container — updated in place every time the scheduler runs.
// applied field is preserved on conflict — scheduler never resets a user's applied flag.
func (s *Store) UpsertRecommendation(ctx context.Context, r *models.Recommendation) error {
	query := `
		INSERT INTO recommendations (
			recommendation_id, cluster_id, namespace, pod_name, container_name,
			status,
			current_cpu_request, current_cpu_limit, current_mem_request, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_request, recommended_cpu_limit,
			recommended_mem_request, recommended_mem_limit,
			applied, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14,
			$15, $16,
			$17, $18, NOW()
		)
		ON CONFLICT (cluster_id, namespace, pod_name, container_name)
		DO UPDATE SET
			status                  = EXCLUDED.status,
			current_cpu_request     = EXCLUDED.current_cpu_request,
			current_cpu_limit       = EXCLUDED.current_cpu_limit,
			current_mem_request     = EXCLUDED.current_mem_request,
			current_mem_limit       = EXCLUDED.current_mem_limit,
			p99_cpu                 = EXCLUDED.p99_cpu,
			p99_mem                 = EXCLUDED.p99_mem,
			recommended_cpu_request = EXCLUDED.recommended_cpu_request,
			recommended_cpu_limit   = EXCLUDED.recommended_cpu_limit,
			recommended_mem_request = EXCLUDED.recommended_mem_request,
			recommended_mem_limit   = EXCLUDED.recommended_mem_limit,
			updated_at              = NOW()
	`
	_, err := s.pool.Exec(ctx, query,
		r.RecommendationID,
		r.ClusterID,
		r.Namespace,
		r.PodName,
		r.ContainerName,
		r.Status,
		r.CurrentCPURequest,
		r.CurrentCPULimit,
		r.CurrentMemRequest,
		r.CurrentMemLimit,
		r.P99CPU,
		r.P99Mem,
		r.RecommendedCPURequest,
		r.RecommendedCPULimit,
		r.RecommendedMemRequest,
		r.RecommendedMemLimit,
		r.Applied,
		r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert recommendation: %w", err)
	}
	return nil
}

// ── Read ──────────────────────────────────────────────────────────────────────

const recommendationColumns = `
	recommendation_id, cluster_id, namespace, pod_name, container_name,
	status,
	current_cpu_request, current_cpu_limit, current_mem_request, current_mem_limit,
	p99_cpu, p99_mem,
	recommended_cpu_request, recommended_cpu_limit,
	recommended_mem_request, recommended_mem_limit,
	applied, created_at, updated_at
`

func scanRecommendation(scanner interface{ Scan(...any) error }, r *models.Recommendation) error {
	return scanner.Scan(
		&r.RecommendationID,
		&r.ClusterID,
		&r.Namespace,
		&r.PodName,
		&r.ContainerName,
		&r.Status,
		&r.CurrentCPURequest,
		&r.CurrentCPULimit,
		&r.CurrentMemRequest,
		&r.CurrentMemLimit,
		&r.P99CPU,
		&r.P99Mem,
		&r.RecommendedCPURequest,
		&r.RecommendedCPULimit,
		&r.RecommendedMemRequest,
		&r.RecommendedMemLimit,
		&r.Applied,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
}

// ListByCluster fetches all recommendations for a cluster ordered by namespace → pod → container.
// Returns empty slice (never nil) so callers can safely serialise to JSON without a nil check.
func (s *Store) ListByCluster(ctx context.Context, clusterID string) ([]*models.Recommendation, error) {
	query := `SELECT ` + recommendationColumns + `
		FROM recommendations
		WHERE cluster_id = $1
		ORDER BY namespace, pod_name, container_name`
	rows, err := s.pool.Query(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list recommendations: %w", err)
	}
	defer rows.Close()

	recommendations := []*models.Recommendation{}
	for rows.Next() {
		r := &models.Recommendation{}
		if err := scanRecommendation(rows, r); err != nil {
			return nil, fmt.Errorf("scan recommendation: %w", err)
		}
		recommendations = append(recommendations, r)
	}
	return recommendations, nil
}

// RecommendationWithCluster is a Recommendation joined with its cluster's human-readable name.
// Used for the cross-cluster view.
type RecommendationWithCluster struct {
	*models.Recommendation
	ClusterName string `json:"cluster_name" db:"cluster_name"`
}

// ListAllWithClusterName fetches every recommendation across every cluster,
// joined with the cluster's name for display. Ordered by biggest CPU request delta first —
// the containers with the most over-provisioned CPU float to the top.
func (s *Store) ListAllWithClusterName(ctx context.Context) ([]*RecommendationWithCluster, error) {
	query := `
		SELECT
			r.recommendation_id, r.cluster_id, r.namespace, r.pod_name, r.container_name,
			r.status,
			r.current_cpu_request, r.current_cpu_limit, r.current_mem_request, r.current_mem_limit,
			r.p99_cpu, r.p99_mem,
			r.recommended_cpu_request, r.recommended_cpu_limit,
			r.recommended_mem_request, r.recommended_mem_limit,
			r.applied, r.created_at, r.updated_at,
			c.cluster_name
		FROM recommendations r
		JOIN clusters c ON r.cluster_id = c.cluster_id
		ORDER BY (r.current_cpu_request - r.recommended_cpu_request) DESC,
		         c.cluster_name, r.namespace, r.pod_name
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all recommendations: %w", err)
	}
	defer rows.Close()

	out := []*RecommendationWithCluster{}
	for rows.Next() {
		item := &RecommendationWithCluster{Recommendation: &models.Recommendation{}}
		if err := rows.Scan(
			&item.RecommendationID,
			&item.ClusterID,
			&item.Namespace,
			&item.PodName,
			&item.ContainerName,
			&item.Status,
			&item.CurrentCPURequest,
			&item.CurrentCPULimit,
			&item.CurrentMemRequest,
			&item.CurrentMemLimit,
			&item.P99CPU,
			&item.P99Mem,
			&item.RecommendedCPURequest,
			&item.RecommendedCPULimit,
			&item.RecommendedMemRequest,
			&item.RecommendedMemLimit,
			&item.Applied,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ClusterName,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation with cluster: %w", err)
		}
		out = append(out, item)
	}
	return out, nil
}
