package models

import "time"

// Cluster status values
const (
	ClusterStatusConnected    = "connected"
	ClusterStatusDisconnected = "disconnected"
)

// LookbackWindow allowed values and default
const (
	LookbackWindow7d  = "7d"
	LookbackWindow10d = "10d"
	LookbackWindow30d = "30d"
	DefaultLookbackWindow = LookbackWindow7d
)

// Cluster represents a registered Kubernetes cluster whose Prometheus endpoint the Hub will query.
type Cluster struct {
	ClusterID       string     `json:"cluster_id"        db:"cluster_id"`
	ClusterName     string     `json:"cluster_name"      db:"cluster_name"`
	PrometheusURL   string     `json:"prometheus_url"    db:"prometheus_url"`
	PrometheusToken string     `json:"-"                 db:"prometheus_token"` // AES-256-GCM encrypted at rest — never exposed in API response
	LookbackWindow  string     `json:"lookback_window"   db:"lookback_window"`  // how far back to query Prometheus — 7d | 10d | 30d
	Status          string     `json:"status"            db:"status"`           // connected | disconnected
	CreatedBy       string     `json:"created_by"        db:"created_by"`       // email of the registering user
	LastSyncedAt    *time.Time `json:"last_synced_at"    db:"last_synced_at"`   // nil if never synced
	CreatedAt       time.Time  `json:"created_at"        db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"        db:"updated_at"`
}
