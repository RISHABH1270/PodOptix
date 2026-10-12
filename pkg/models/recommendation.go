package models

import "time"

// Recommendation status values
const (
	RecommendationStatusNewService = "new_service" // not enough data yet — check back after cluster's lookback window
	RecommendationStatusReady      = "ready"       // p99 computed — recommendation is available
)

// Workload kind values — map to K8s PodController types.
// Resolved from kube_pod_owner → kube_replicaset_owner during collection.
const (
	WorkloadKindDeployment  = "Deployment"
	WorkloadKindStatefulSet = "StatefulSet"
	WorkloadKindDaemonSet   = "DaemonSet"
	WorkloadKindPod         = "Pod" // fallback for bare pods / unknown owners
)

// Recommendation represents a resource recommendation for a single CONTAINER within a WORKLOAD.
// A "workload" is the K8s PodController (Deployment / StatefulSet / DaemonSet) that owns pods.
// We aggregate across replicas — one row per (cluster, namespace, workload, container).
//
// CPU in millicores (1000m = 1 core) · Memory in MiB (1024 MiB = 1 GiB).
//
// K8s has two levers per resource:
//   requests → scheduler reserves this (pod is guaranteed at least this much)
//   limits   → hard ceiling (CPU gets throttled, pod gets OOMKilled on memory overrun)
//
// Formula (same for CPU and memory):
//   For each timestamp t over the lookback window:
//     usage(t) = MAX over all replicas of container_usage(replica, t)
//   p99     = p99 of usage(t) over all t
//   request = ceil(p99)
//   limit   = ceil(p99 × 2)
//
// FirstMissedAt is NULL when the workload was seen in the most recent scheduler run.
// When set, it records when the scheduler FIRST noticed the workload was missing
// from Prometheus — NOT when the workload was actually deleted (we can't know that).
// The real deletion could have happened any time between the previous scheduler
// run and this one. Once set, it's not updated on subsequent misses (we keep the
// first detection). The next upsert resets it to NULL if the workload reappears.
type Recommendation struct {
	RecommendationID string `json:"recommendation_id" db:"recommendation_id"`
	ClusterID        string `json:"cluster_id"        db:"cluster_id"`
	Status           string `json:"status"            db:"status"`

	// Workload identity
	Namespace     string `json:"namespace"      db:"namespace"`
	WorkloadKind  string `json:"workload_kind"  db:"workload_kind"`  // Deployment | StatefulSet | DaemonSet | Pod
	WorkloadName  string `json:"workload_name"  db:"workload_name"`  // e.g. "auth-service"
	ContainerName string `json:"container_name" db:"container_name"` // e.g. "auth" or "envoy"
	ReplicaCount  int    `json:"replica_count"  db:"replica_count"`  // how many replicas aggregated this run

	// Current state (what the workload is running right now)
	CurrentCPURequest int `json:"current_cpu_request" db:"current_cpu_request"` // millicores
	CurrentCPULimit   int `json:"current_cpu_limit"   db:"current_cpu_limit"`   // millicores
	CurrentMemRequest int `json:"current_mem_request" db:"current_mem_request"` // MiB
	CurrentMemLimit   int `json:"current_mem_limit"   db:"current_mem_limit"`   // MiB

	// Raw p99 values (max across replicas, then p99 across time)
	P99CPU float64 `json:"p99_cpu" db:"p99_cpu"` // millicores
	P99Mem float64 `json:"p99_mem" db:"p99_mem"` // MiB

	// Recommendations
	RecommendedCPURequest int `json:"recommended_cpu_request" db:"recommended_cpu_request"` // ceil(p99_cpu)
	RecommendedCPULimit   int `json:"recommended_cpu_limit"   db:"recommended_cpu_limit"`   // ceil(p99_cpu × 2)
	RecommendedMemRequest int `json:"recommended_mem_request" db:"recommended_mem_request"` // ceil(p99_mem)
	RecommendedMemLimit   int `json:"recommended_mem_limit"   db:"recommended_mem_limit"`   // ceil(p99_mem × 2)

	Applied bool `json:"applied" db:"applied"` // toggled true when applied to cluster — drives savings math

	// Tombstone — NULL means alive, timestamp means the scheduler first noticed
	// this workload was missing at that time. *time.Time to allow NULL
	// (JSON serialises to null when unset).
	FirstMissedAt *time.Time `json:"first_missed_at,omitempty" db:"first_missed_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// WorkloadKey uniquely identifies a workload-container row within a cluster.
// Used by the scheduler to tell the store "these are the keys I saw this run — mark anything else orphaned".
type WorkloadKey struct {
	Namespace     string
	WorkloadKind  string
	WorkloadName  string
	ContainerName string
}
