package models

import "time"

// Cluster represents a registered Kubernetes cluster
// whose Prometheus endpoint the Hub will query.
type Cluster struct {
	ID             string    `json:"id"              db:"id"`
	Name           string    `json:"name"            db:"name"`
	PrometheusURL  string    `json:"prometheus_url"  db:"prometheus_url"`
	Token          string    `json:"-"               db:"token"`           // never exposed in API response
	LookbackWindow string    `json:"lookback_window" db:"lookback_window"` // how far back to look e.g. "7d"
	CreatedAt      time.Time `json:"created_at"      db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"      db:"updated_at"`
}
