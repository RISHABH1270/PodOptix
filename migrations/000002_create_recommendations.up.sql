-- Migration 002: Create recommendations table
-- One row per container — updated in place daily by the scheduler.
-- CPU stored in millicores (1000m = 1 core), Memory in MiB (1024Mi = 1Gi).
--
-- K8s has two levers per resource:
--   requests → what the scheduler reserves (the pod is guaranteed this much)
--   limits   → the hard ceiling (CPU gets throttled, memory gets OOMKilled above this)
--
-- Formula (same for CPU and memory):
--   request = ceil(p99)
--   limit   = ceil(p99 × 2)

CREATE TABLE IF NOT EXISTS recommendations (
    recommendation_id       VARCHAR(36)   PRIMARY KEY,
    cluster_id              VARCHAR(36)   NOT NULL REFERENCES clusters(cluster_id),
    status                  VARCHAR(20)   NOT NULL DEFAULT 'new_service',   -- new_service | ready
    namespace               VARCHAR(255)  NOT NULL,
    pod_name                VARCHAR(255)  NOT NULL,
    container_name          VARCHAR(255)  NOT NULL,

    -- Current state (what the cluster is running right now)
    current_cpu_request     INTEGER       NOT NULL DEFAULT 0,   -- millicores
    current_cpu_limit       INTEGER       NOT NULL DEFAULT 0,   -- millicores
    current_mem_request     INTEGER       NOT NULL DEFAULT 0,   -- MiB
    current_mem_limit       INTEGER       NOT NULL DEFAULT 0,   -- MiB

    -- Computed p99 (raw percentile values from Prometheus usage data)
    p99_cpu                 FLOAT         NOT NULL DEFAULT 0,   -- millicores
    p99_mem                 FLOAT         NOT NULL DEFAULT 0,   -- MiB

    -- Recommendations (what PodOptix suggests the cluster should run)
    recommended_cpu_request INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_cpu)
    recommended_cpu_limit   INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_cpu × 2)
    recommended_mem_request INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_mem)
    recommended_mem_limit   INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_mem × 2)

    applied                 BOOLEAN       NOT NULL DEFAULT FALSE,  -- true when recommendation applied to cluster — enables cost savings calculation
    created_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- ensures one recommendation per container per cluster all the time
    UNIQUE (cluster_id, namespace, pod_name, container_name)
);

-- Index for fast lookup by cluster
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);
