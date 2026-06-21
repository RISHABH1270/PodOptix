-- Migration 002: Create recommendations table
-- Stores p99-based resource recommendations per container

CREATE TABLE IF NOT EXISTS recommendations (
    id                    VARCHAR(36)   PRIMARY KEY,                         -- UUID
    cluster_id            VARCHAR(36)   NOT NULL REFERENCES clusters(id),    -- foreign key → clusters
    status                VARCHAR(20)   NOT NULL DEFAULT 'new_service',      -- new_service | ready
    namespace             VARCHAR(255)  NOT NULL,                            -- e.g. "payments-ns"
    pod_name              VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api-7d9f"
    container_name        VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api"
    current_cpu_limit     VARCHAR(20)   NOT NULL,                            -- e.g. "1000m"
    current_mem_limit     VARCHAR(20)   NOT NULL,                            -- e.g. "1024Mi"
    p99_cpu               FLOAT         NOT NULL DEFAULT 0,                  -- p99 CPU in millicores (0 if new_service)
    p99_mem               FLOAT         NOT NULL DEFAULT 0,                  -- p99 Memory in MiB (0 if new_service)
    recommended_cpu_limit VARCHAR(20)   NOT NULL DEFAULT '',                 -- p99_cpu x 2 (empty if new_service)
    recommended_mem_limit VARCHAR(20)   NOT NULL DEFAULT '',                 -- p99_mem x 2 (empty if new_service)
    window                VARCHAR(10)   NOT NULL,                            -- data window e.g. "7d"
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Index for fast lookup of recommendations by cluster
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);
