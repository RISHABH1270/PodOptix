package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrClusterNameTaken is returned by SaveCluster when another cluster already
// has the same cluster_name. Handler uses errors.Is() to map to 409 Conflict.
var ErrClusterNameTaken = errors.New("cluster name already in use")

// ── Create ────────────────────────────────────────────────────────────────────

// SaveCluster inserts a cluster. Returns ErrClusterNameTaken if a cluster with
// the same cluster_name already exists (UNIQUE constraint violation).
func (s *Store) SaveCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		INSERT INTO clusters (cluster_id, cluster_name, prometheus_url, prometheus_token, lookback_window, status, created_by, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := s.pool.Exec(ctx, query,
		c.ClusterID,
		c.ClusterName,
		c.PrometheusURL,
		c.PrometheusToken,
		c.LookbackWindow,
		c.Status,
		c.CreatedBy,
		nil,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		// SQLSTATE 23505 = unique_violation. Only one UNIQUE constraint exists
		// on clusters (cluster_name), so a hit means the name is taken.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrClusterNameTaken
		}
		return fmt.Errorf("save cluster: %w", err)
	}
	return nil
}

// ── Read ──────────────────────────────────────────────────────────────────────

func (s *Store) GetCluster(ctx context.Context, clusterID string) (*models.Cluster, error) {
	query := `
		SELECT cluster_id, cluster_name, prometheus_url, prometheus_token, lookback_window, status, created_by, last_synced_at, created_at, updated_at
		FROM clusters
		WHERE cluster_id = $1
	`
	c := &models.Cluster{}
	err := s.pool.QueryRow(ctx, query, clusterID).Scan(
		&c.ClusterID,
		&c.ClusterName,
		&c.PrometheusURL,
		&c.PrometheusToken,
		&c.LookbackWindow,
		&c.Status,
		&c.CreatedBy,
		&c.LastSyncedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}
	return c, nil
}

func (s *Store) ListClusters(ctx context.Context) ([]*models.Cluster, error) {
	query := `
		SELECT cluster_id, cluster_name, prometheus_url, prometheus_token, lookback_window, status, created_by, last_synced_at, created_at, updated_at
		FROM clusters
		ORDER BY created_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	clusters := []*models.Cluster{} // never nil — handlers serialise to [] not null
	for rows.Next() {
		c := &models.Cluster{}
		if err := rows.Scan(
			&c.ClusterID,
			&c.ClusterName,
			&c.PrometheusURL,
			&c.PrometheusToken,
			&c.LookbackWindow,
			&c.Status,
			&c.CreatedBy,
			&c.LastSyncedAt,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

// ── Update ────────────────────────────────────────────────────────────────────

func (s *Store) UpdateCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		UPDATE clusters
		SET cluster_name = $1, prometheus_url = $2, prometheus_token = $3, lookback_window = $4, updated_at = NOW()
		WHERE cluster_id = $5
	`
	tag, err := s.pool.Exec(ctx, query,
		c.ClusterName,
		c.PrometheusURL,
		c.PrometheusToken,
		c.LookbackWindow,
		c.ClusterID,
	)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cluster not found: %s", c.ClusterID)
	}
	return nil
}

// UpdateClusterHealth updates status and last_synced_at after a collection run.
func (s *Store) UpdateClusterHealth(ctx context.Context, clusterID string, status string, collectedAt time.Time) error {
	query := `
		UPDATE clusters
		SET status = $1, last_synced_at = $2, updated_at = NOW()
		WHERE cluster_id = $3
	`
	_, err := s.pool.Exec(ctx, query, status, collectedAt, clusterID)
	if err != nil {
		return fmt.Errorf("update cluster health: %w", err)
	}
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

func (s *Store) DeleteCluster(ctx context.Context, clusterID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM clusters WHERE cluster_id = $1`, clusterID)
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cluster not found: %s", clusterID)
	}
	return nil
}
