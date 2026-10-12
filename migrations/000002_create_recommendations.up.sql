-- Migration 002: Create recommendations table
-- One row per CONTAINER per WORKLOAD per CLUSTER — updated in place by the scheduler.
-- CPU stored in millicores (1000m = 1 core), Memory in MiB (1024 MiB = 1 GiB).
--
-- Why workload-level, not pod-level:
--   A Deployment with 3 replicas creates 3 pods with ephemeral random suffixes
--   (auth-service-fgerg8, auth-service-4kjni4, ...). Storing per-pod rows means
--   stale rows forever as pods churn + divergent recommendations across replicas
--   that should be sized identically. We store per workload instead.
--
-- K8s has two levers per resource:
--   requests → what the scheduler reserves (the pod is guaranteed this much)
--   limits   → the hard ceiling (CPU throttled, pod OOMKilled on memory overrun)
--
-- Formula (same for CPU and memory):
--   For each timestamp t over the lookback window:
--     usage(t) = MAX over all replicas of (container_usage(replica, t))
--   p99     = p99 of usage(t) over all t
--   request = ceil(p99)
--   limit   = ceil(p99 × 2)

CREATE TABLE IF NOT EXISTS recommendations (
    recommendation_id       VARCHAR(36)   PRIMARY KEY,
    cluster_id              VARCHAR(36)   NOT NULL REFERENCES clusters(cluster_id) ON DELETE CASCADE,
    status                  VARCHAR(20)   NOT NULL DEFAULT 'new_service',   -- new_service | ready

    -- Workload identity (resolved from pod → ReplicaSet → Deployment chain)
    namespace               VARCHAR(255)  NOT NULL,
    workload_kind           VARCHAR(50)   NOT NULL,                         -- Deployment | StatefulSet | DaemonSet | Pod
    workload_name           VARCHAR(255)  NOT NULL,                         -- e.g. "auth-service" (NOT the pod name)
    container_name          VARCHAR(255)  NOT NULL,                         -- a workload can have multiple containers (main + sidecars)
    replica_count           INTEGER       NOT NULL DEFAULT 1,               -- how many replicas we aggregated across this run

    -- Current state (what the workload is running right now — matches live kube-state-metrics)
    current_cpu_request     INTEGER       NOT NULL DEFAULT 0,   -- millicores
    current_cpu_limit       INTEGER       NOT NULL DEFAULT 0,   -- millicores
    current_mem_request     INTEGER       NOT NULL DEFAULT 0,   -- MiB
    current_mem_limit       INTEGER       NOT NULL DEFAULT 0,   -- MiB

    -- Computed p99 (max across replicas → p99 across time)
    p99_cpu                 FLOAT         NOT NULL DEFAULT 0,   -- millicores
    p99_mem                 FLOAT         NOT NULL DEFAULT 0,   -- MiB

    -- Recommendations
    recommended_cpu_request INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_cpu)
    recommended_cpu_limit   INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_cpu × 2)
    recommended_mem_request INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_mem)
    recommended_mem_limit   INTEGER       NOT NULL DEFAULT 0,   -- ceil(p99_mem × 2)

    applied                 BOOLEAN       NOT NULL DEFAULT FALSE,  -- user toggles true after applying — drives savings math

    -- Tombstone: NULL = workload is alive, timestamp = when the scheduler FIRST noticed
    -- the workload was missing from Prometheus. This is NOT the time the workload was
    -- actually deleted — we have no way to know that. The real deletion could have
    -- happened anywhere between the previous scheduler run and this one.
    -- Rows are NEVER auto-deleted — operator reviews orphans in the dashboard and
    -- deletes them explicitly. If the workload comes back later (deleted + recreated),
    -- the next upsert clears first_missed_at back to NULL. Already-set timestamps are
    -- NOT updated on subsequent misses — we keep the FIRST detection.
    first_missed_at         TIMESTAMPTZ,

    created_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- One row per container-within-workload per cluster
    UNIQUE (cluster_id, namespace, workload_kind, workload_name, container_name)
);

-- Fast lookup by cluster (every dashboard query filters on this)
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);

-- Fast lookup of orphaned rows for the "orphaned" dashboard section
CREATE INDEX IF NOT EXISTS idx_recommendations_orphaned
    ON recommendations(cluster_id)
    WHERE first_missed_at IS NOT NULL;
