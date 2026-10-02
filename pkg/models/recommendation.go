package models

import "time"

// Recommendation status values
const (
	RecommendationStatusNewService = "new_service" // not enough data yet — check back after cluster's lookback window
	RecommendationStatusReady      = "ready"       // p99 computed — recommendation is available
)

// Recommendation represents a resource recommendation for a single container.
// One row per container — updated in place every day by the scheduler.
// CPU in millicores (1000m = 1 core) · Memory in MiB (1024 MiB = 1 GiB).
//
// K8s has two levers per resource:
//   requests → scheduler reserves this (pod is guaranteed at least this much)
//   limits   → hard ceiling (CPU gets throttled, memory gets OOMKilled above this)
//
// Formula (same for CPU and memory):
//   request = ceil(p99)
//   limit   = ceil(p99 × 2)
type Recommendation struct {
	RecommendationID      string    `json:"recommendation_id"       db:"recommendation_id"`
	ClusterID             string    `json:"cluster_id"              db:"cluster_id"`
	Namespace             string    `json:"namespace"               db:"namespace"`
	PodName               string    `json:"pod_name"                db:"pod_name"`
	ContainerName         string    `json:"container_name"          db:"container_name"`
	Status                string    `json:"status"                  db:"status"`

	// Current state (what the cluster is running right now)
	CurrentCPURequest     int       `json:"current_cpu_request"     db:"current_cpu_request"`     // millicores
	CurrentCPULimit       int       `json:"current_cpu_limit"       db:"current_cpu_limit"`       // millicores
	CurrentMemRequest     int       `json:"current_mem_request"     db:"current_mem_request"`     // MiB
	CurrentMemLimit       int       `json:"current_mem_limit"       db:"current_mem_limit"`       // MiB

	// Raw p99 values from Prometheus
	P99CPU                float64   `json:"p99_cpu"                 db:"p99_cpu"`                 // millicores
	P99Mem                float64   `json:"p99_mem"                 db:"p99_mem"`                 // MiB

	// Recommendations
	RecommendedCPURequest int       `json:"recommended_cpu_request" db:"recommended_cpu_request"` // ceil(p99_cpu)
	RecommendedCPULimit   int       `json:"recommended_cpu_limit"   db:"recommended_cpu_limit"`   // ceil(p99_cpu × 2)
	RecommendedMemRequest int       `json:"recommended_mem_request" db:"recommended_mem_request"` // ceil(p99_mem)
	RecommendedMemLimit   int       `json:"recommended_mem_limit"   db:"recommended_mem_limit"`   // ceil(p99_mem × 2)

	Applied               bool      `json:"applied"                 db:"applied"`                 // toggled true when applied — enables savings calc
	CreatedAt             time.Time `json:"created_at"              db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"              db:"updated_at"`
}
