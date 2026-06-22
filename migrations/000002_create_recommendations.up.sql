-- Migration 002: Create recommendations table
-- Stores p99-based resource recommendations per container
-- All CPU values stored in millicores (integer). 1000m = 1 core
-- All Memory values stored in Mebibytes (integer). 1024Mi = 1Gi

CREATE TABLE IF NOT EXISTS recommendations (
    id                    VARCHAR(36)   PRIMARY KEY,                         -- UUID
    cluster_id            VARCHAR(36)   NOT NULL REFERENCES clusters(id),    -- foreign key → clusters
    status                VARCHAR(20)   NOT NULL DEFAULT 'new_service',      -- new_service | ready
    namespace             VARCHAR(255)  NOT NULL,                            -- e.g. "payments-ns"
    pod_name              VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api-7d9f"
    container_name        VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api"
    current_cpu_limit     INTEGER       NOT NULL DEFAULT 0,                  -- millicores e.g. 1000
    current_mem_limit     INTEGER       NOT NULL DEFAULT 0,                  -- MiB e.g. 1024
    p99_cpu               FLOAT         NOT NULL DEFAULT 0,                  -- p99 CPU in millicores
    p99_mem               FLOAT         NOT NULL DEFAULT 0,                  -- p99 Memory in MiB
    recommended_cpu_limit INTEGER       NOT NULL DEFAULT 0,                  -- millicores e.g. 241
    recommended_mem_limit INTEGER       NOT NULL DEFAULT 0,                  -- MiB e.g. 360
    lookback_window                VARCHAR(10)   NOT NULL,                            -- data lookback_window e.g. "7d"
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Index for fast lookup of recommendations by cluster
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);
