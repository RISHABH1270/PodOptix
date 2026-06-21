-- Migration 002: Create recommendations table
-- Stores p99-based resource recommendations per container

CREATE TABLE IF NOT EXISTS recommendations (
    id                    VARCHAR(36)   PRIMARY KEY,                         -- UUID
    cluster_id            VARCHAR(36)   NOT NULL REFERENCES clusters(id),    -- foreign key → clusters
    namespace             VARCHAR(255)  NOT NULL,                            -- e.g. "payments-ns"
    pod_name              VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api-7d9f"
    container_name        VARCHAR(255)  NOT NULL,                            -- e.g. "payment-api"
    current_cpu_limit     VARCHAR(20)   NOT NULL,                            -- e.g. "1000m"
    current_mem_limit     VARCHAR(20)   NOT NULL,                            -- e.g. "1024Mi"
    p99_cpu               FLOAT         NOT NULL,                            -- p99 CPU in millicores
    p99_mem               FLOAT         NOT NULL,                            -- p99 Memory in MiB
    recommended_cpu_limit VARCHAR(20)   NOT NULL,                            -- p99_cpu x 2 e.g. "241m"
    recommended_mem_limit VARCHAR(20)   NOT NULL,                            -- p99_mem x 2 e.g. "360Mi"
    window                VARCHAR(10)   NOT NULL,                            -- data window e.g. "7d"
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Index for fast lookup of recommendations by cluster
CREATE INDEX IF NOT EXISTS idx_recommendations_cluster_id ON recommendations(cluster_id);
