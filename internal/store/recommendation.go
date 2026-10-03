package store

import (
	"context"
	"fmt"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// ── Create / Update (Upsert) ──────────────────────────────────────────────────

// UpsertRecommendation inserts a new recommendation or updates the existing one.
// One row per workload-container — updated in place every time the scheduler runs.
// `applied` is preserved on conflict (scheduler never resets a user's applied flag).
// `orphaned_at` is cleared on every upsert — a workload we just saw is alive by definition.
func (s *Store) UpsertRecommendation(ctx context.Context, r *models.Recommendation) error {
	query := `
		INSERT INTO recommendations (
			recommendation_id, cluster_id, namespace, workload_kind, workload_name, container_name, replica_count,
			status,
			current_cpu_request, current_cpu_limit, current_mem_request, current_mem_limit,
			p99_cpu, p99_mem,
			recommended_cpu_request, recommended_cpu_limit,
			recommended_mem_request, recommended_mem_limit,
			applied, orphaned_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8,
			$9, $10, $11, $12,
			$13, $14,
			$15, $16,
			$17, $18,
			$19, NULL, $20, NOW()
		)
		ON CONFLICT (cluster_id, namespace, workload_kind, workload_name, container_name)
		DO UPDATE SET
			replica_count           = EXCLUDED.replica_count,
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
			orphaned_at             = NULL,
			updated_at              = NOW()
	`
	_, err := s.pool.Exec(ctx, query,
		r.RecommendationID,
		r.ClusterID,
		r.Namespace,
		r.WorkloadKind,
		r.WorkloadName,
		r.ContainerName,
		r.ReplicaCount,
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
	recommendation_id, cluster_id, namespace, workload_kind, workload_name, container_name, replica_count,
	status,
	current_cpu_request, current_cpu_limit, current_mem_request, current_mem_limit,
	p99_cpu, p99_mem,
	recommended_cpu_request, recommended_cpu_limit,
	recommended_mem_request, recommended_mem_limit,
	applied, orphaned_at, created_at, updated_at
`

func scanRecommendation(scanner interface{ Scan(...any) error }, r *models.Recommendation) error {
	return scanner.Scan(
		&r.RecommendationID,
		&r.ClusterID,
		&r.Namespace,
		&r.WorkloadKind,
		&r.WorkloadName,
		&r.ContainerName,
		&r.ReplicaCount,
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
		&r.OrphanedAt,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
}

// ListByCluster fetches all recommendations for a cluster ordered by namespace → workload → container.
// Returns empty slice (never nil) so callers can safely serialise to JSON without a nil check.
func (s *Store) ListByCluster(ctx context.Context, clusterID string) ([]*models.Recommendation, error) {
	query := `SELECT ` + recommendationColumns + `
		FROM recommendations
		WHERE cluster_id = $1
		ORDER BY namespace, workload_name, container_name`
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
// the workloads with the most over-provisioned CPU float to the top.
func (s *Store) ListAllWithClusterName(ctx context.Context) ([]*RecommendationWithCluster, error) {
	query := `
		SELECT
			r.recommendation_id, r.cluster_id, r.namespace, r.workload_kind, r.workload_name,
			r.container_name, r.replica_count,
			r.status,
			r.current_cpu_request, r.current_cpu_limit, r.current_mem_request, r.current_mem_limit,
			r.p99_cpu, r.p99_mem,
			r.recommended_cpu_request, r.recommended_cpu_limit,
			r.recommended_mem_request, r.recommended_mem_limit,
			r.applied, r.orphaned_at, r.created_at, r.updated_at,
			c.cluster_name
		FROM recommendations r
		JOIN clusters c ON r.cluster_id = c.cluster_id
		ORDER BY (r.current_cpu_request - r.recommended_cpu_request) DESC,
		         c.cluster_name, r.namespace, r.workload_name
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
			&item.WorkloadKind,
			&item.WorkloadName,
			&item.ContainerName,
			&item.ReplicaCount,
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
			&item.OrphanedAt,
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

// ── Tombstone / Delete ────────────────────────────────────────────────────────

// MarkOrphaned stamps orphaned_at=NOW() on every row in the cluster whose
// (namespace, workload_kind, workload_name, container_name) is NOT in seenKeys —
// i.e. the scheduler just ran and these workloads weren't observed in Prometheus.
//
// SAFETY GATE: if seenKeys is empty, this is a no-op (returns 0, nil). An empty
// scheduler run usually means Prometheus hiccupped or kube-state-metrics went
// missing — NOT that every workload in the cluster disappeared. We refuse to
// stamp everything orphaned in that case.
//
// Rows already stamped (orphaned_at IS NOT NULL) are left alone — their timestamp
// records WHEN they first went missing, not the latest run.
//
// Returns the number of rows newly stamped.
func (s *Store) MarkOrphaned(ctx context.Context, clusterID string, seenKeys []models.WorkloadKey) (int, error) {
	if len(seenKeys) == 0 {
		return 0, nil
	}

	// UNNEST four parallel arrays into a seen(ns, kind, name, ctr) table,
	// then NOT EXISTS to mark everything else orphaned.
	namespaces := make([]string, len(seenKeys))
	kinds := make([]string, len(seenKeys))
	names := make([]string, len(seenKeys))
	containers := make([]string, len(seenKeys))
	for i, k := range seenKeys {
		namespaces[i] = k.Namespace
		kinds[i] = k.WorkloadKind
		names[i] = k.WorkloadName
		containers[i] = k.ContainerName
	}

	query := `
		UPDATE recommendations r
		SET orphaned_at = NOW()
		WHERE r.cluster_id = $1
		  AND r.orphaned_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1
		    FROM UNNEST($2::text[], $3::text[], $4::text[], $5::text[])
		         AS seen(ns, kind, name, ctr)
		    WHERE seen.ns   = r.namespace
		      AND seen.kind = r.workload_kind
		      AND seen.name = r.workload_name
		      AND seen.ctr  = r.container_name
		  )
	`
	tag, err := s.pool.Exec(ctx, query, clusterID, namespaces, kinds, names, containers)
	if err != nil {
		return 0, fmt.Errorf("mark orphaned: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// DeleteOrphaned hard-deletes every orphaned row for a cluster.
// Called from the dashboard "delete all orphans" bulk action.
// Returns the number of rows deleted.
func (s *Store) DeleteOrphaned(ctx context.Context, clusterID string) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM recommendations WHERE cluster_id = $1 AND orphaned_at IS NOT NULL`,
		clusterID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete orphaned: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// DeleteRecommendation hard-deletes a single recommendation by id.
// Called from the dashboard per-row delete action. The caller is expected
// to have scoped the request to the cluster_id already.
func (s *Store) DeleteRecommendation(ctx context.Context, recommendationID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM recommendations WHERE recommendation_id = $1`,
		recommendationID,
	)
	if err != nil {
		return fmt.Errorf("delete recommendation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("recommendation not found: %s", recommendationID)
	}
	return nil
}
