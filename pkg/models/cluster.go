package models

import "time"

// Cluster health status values
const (
	ClusterStatusPending   = "pending"   // newly registered — never synced yet
	ClusterStatusHealthy   = "healthy"   // last sync succeeded
	ClusterStatusUnhealthy = "unhealthy" // last sync failed — Prometheus unreachable
)

// Cluster represents a registered Kubernetes cluster whose Prometheus endpoint the Hub will query.
type Cluster struct {
	ClusterID       string     `json:"cluster_id"        db:"cluster_id"`
	Name            string     `json:"name"              db:"name"`
	PrometheusURL   string     `json:"prometheus_url"    db:"prometheus_url"`
	Token           string     `json:"-"                 db:"token"`            // AES-256-GCM encrypted at rest - never exposed in API response
	LookbackWindow  string     `json:"lookback_window"   db:"lookback_window"`  // how far back to look e.g. "7d"
	Status          string     `json:"status"            db:"status"`           // healthy | unhealthy
	LastSyncedAt    *time.Time `json:"last_synced_at"    db:"last_synced_at"`   // nil if never collected
	CreatedAt       time.Time  `json:"created_at"        db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"        db:"updated_at"`
}
