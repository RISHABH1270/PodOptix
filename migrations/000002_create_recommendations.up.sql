-- Migration 002: Create recommendations table
-- One row per container — updated in place daily by the scheduler.
-- CPU stored in millicores (1000m = 1 core), Memory in MiB (1024Mi = 1Gi).

CREATE TABLE IF NOT EXISTS recommendations (
    recommendation_id     VARCHAR(36)   PRIMARY KEY,
    cluster_id            VARCHAR(36)   NOT NULL REFERENCES clusters(cluster_id),
    status                VARCHAR(20)   NOT NULL DEFAULT 'new_service',      -- new_service | ready
    namespace             VARCHAR(255)  NOT NULL,
    pod_name              VARCHAR(255)  NOT NULL,
    container_name        VARCHAR(255)  NOT NULL,
    current_cpu_limit     INTEGER       NOT NULL DEFAULT 0,                  -- millicores
    current_mem_limit     INTEGER       NOT NULL DEFAULT 0,                  -- MiB
    p99_cpu               FLOAT         NOT NULL DEFAULT 0,                  -- millicores
    p99_mem               FLOAT         NOT NULL DEFAULT 0,                  -- MiB
    recommended_cpu_limit INTEGER       NOT NULL DEFAULT 0,                  -- p99 x 2
    recommended_mem_limit INTEGER       NOT NULL DEFAULT 0,                  -- p99 x 2
    lookback_window       VARCHAR(10)   NOT NULL,
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- ensures one recommendation per container per cluster all the time
    UNIQUE (cluster_id, namespace, pod_name, container_name)
);

-- Index for fast lookup by cluster
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);
