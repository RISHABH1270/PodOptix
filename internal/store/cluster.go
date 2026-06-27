package store

import (
	"context"
	"fmt"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// SaveCluster inserts a new cluster into the database.
func (s *Store) SaveCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		INSERT INTO clusters (cluster_id, name, prometheus_url, token, lookback_window, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.pool.Exec(ctx, query,
		c.ClusterID,
		c.Name,
		c.PrometheusURL,
		c.Token,
		c.LookbackWindow,
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
		SELECT cluster_id, name, prometheus_url, token, lookback_window, created_at, updated_at
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
		SELECT cluster_id, name, prometheus_url, token, lookback_window, created_at, updated_at
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

// UpdateCluster updates the updated_at timestamp when a cluster is modified.
func (s *Store) UpdateCluster(ctx context.Context, c *models.Cluster) error {
	query := `
		UPDATE clusters
		SET name = $1, prometheus_url = $2, token = $3, lookback_window = $4, updated_at = $5
		WHERE cluster_id = $6
	`
	_, err := s.pool.Exec(ctx, query,
		c.Name,
		c.PrometheusURL,
		c.Token,
		c.LookbackWindow,
		time.Now(),
		c.ClusterID,
	)
	if err != nil {
		return fmt.Errorf("update cluster: %w", err)
	}
	return nil
}
