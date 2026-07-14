package store

import (
	"context"
	"fmt"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// note: time imported for UpdateClusterHealth parameter

// SaveCluster inserts a new cluster into the database.
func (s *Store) SaveCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		INSERT INTO clusters (cluster_id, name, prometheus_url, token, lookback_window, status, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.pool.Exec(ctx, query,
		c.ClusterID,
		c.Name,
		c.PrometheusURL,
		c.Token,
		c.LookbackWindow,
		models.ClusterStatusPending, // new clusters start as pending — never synced yet
		nil,                         // last_synced_at NULL — never synced yet
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save cluster: %w", err)
	}
	return nil
}

// GetCluster fetches a single cluster by its ID.
func (s *Store) GetCluster(ctx context.Context, id string) (*models.Cluster, error) {
	query := `
		SELECT cluster_id, name, prometheus_url, token, lookback_window, status, last_synced_at, created_at, updated_at
		FROM clusters
		WHERE cluster_id = $1
	`
	row := s.pool.QueryRow(ctx, query, id)

	c := &models.Cluster{}
	err := row.Scan(
		&c.ClusterID,
		&c.Name,
		&c.PrometheusURL,
		&c.Token,
		&c.LookbackWindow,
		&c.Status,
		&c.LastSyncedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}
	return c, nil
}

// ListClusters fetches all registered clusters.
func (s *Store) ListClusters(ctx context.Context) ([]*models.Cluster, error) {
	query := `
		SELECT cluster_id, name, prometheus_url, token, lookback_window, status, last_synced_at, created_at, updated_at
		FROM clusters
		ORDER BY created_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		c := &models.Cluster{}
		err := rows.Scan(
			&c.ClusterID,
			&c.Name,
			&c.PrometheusURL,
			&c.Token,
			&c.LookbackWindow,
			&c.Status,
			&c.LastSyncedAt,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

// DeleteCluster removes a cluster by its ID.
func (s *Store) DeleteCluster(ctx context.Context, id string) error {
	query := `DELETE FROM clusters WHERE cluster_id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	return nil
}

// UpdateCluster updates cluster details.
func (s *Store) UpdateCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		UPDATE clusters
		SET name = $1, prometheus_url = $2, token = $3, lookback_window = $4, updated_at = NOW()
		WHERE cluster_id = $5
	`
	_, err := s.pool.Exec(ctx, query,
		c.Name,
		c.PrometheusURL,
		c.Token,
		c.LookbackWindow,
		c.ClusterID,
	)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
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
