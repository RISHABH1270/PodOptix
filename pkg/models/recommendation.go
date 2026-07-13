package models

import "time"

// Recommendation status values
const (
	RecommendationStatusNewService = "new_service"  // not enough data yet — check back after 7 days
	RecommendationStatusReady      = "ready"        // p99 computed — recommendation is available
)

// Recommendation represents a resource limit recommendation for a single container.
// One row per container — updated in place every day by the scheduler.
// CPU stored in millicores (1000m = 1 core), Memory in MiB (1024Mi = 1Gi).
type Recommendation struct {
	RecommendationID string  `json:"recommendation_id" db:"recommendation_id"`
	ClusterID        string  `json:"cluster_id"        db:"cluster_id"`
	Namespace        string  `json:"namespace"         db:"namespace"`
	PodName          string  `json:"pod_name"          db:"pod_name"`
	ContainerName    string  `json:"container_name"    db:"container_name"`
	Status           string  `json:"status"            db:"status"`  // new_service | ready

	CurrentCPULimit  int     `json:"current_cpu_limit" db:"current_cpu_limit"` // millicores
	CurrentMemLimit  int     `json:"current_mem_limit" db:"current_mem_limit"` // MiB

	P99CPU           float64 `json:"p99_cpu" db:"p99_cpu"` // millicores
	P99Mem           float64 `json:"p99_mem" db:"p99_mem"` // MiB

	RecommendedCPULimit int  `json:"recommended_cpu_limit" db:"recommended_cpu_limit"` // p99
	RecommendedMemLimit int  `json:"recommended_mem_limit" db:"recommended_mem_limit"` // p99

	LookbackWindow   string  `json:"lookback_window" db:"lookback_window"`

	CreatedAt        time.Time `json:"created_at"      db:"created_at"` // when first generated
	UpdatedAt        time.Time `json:"updated_at"      db:"updated_at"` // last recalculated
}
