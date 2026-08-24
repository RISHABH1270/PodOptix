-- Migration 001: Create clusters table

CREATE TABLE IF NOT EXISTS clusters (
    cluster_id       VARCHAR(36)   PRIMARY KEY,                    -- UUID
    cluster_name     VARCHAR(100)  NOT NULL UNIQUE,                -- max 100 chars
    prometheus_url   VARCHAR(500)  NOT NULL,                       -- Prometheus HTTP endpoint
    prometheus_token TEXT          NOT NULL,                       -- AES-256-GCM encrypted token
    lookback_window  VARCHAR(10)   NOT NULL DEFAULT '7d',          -- 7d | 10d | 30d
    status           VARCHAR(20)   NOT NULL DEFAULT 'disconnected',-- connected | disconnected
    created_by       VARCHAR(36),                                  -- user_id audit trail (nullable for legacy)
    last_synced_at   TIMESTAMPTZ,                                  -- NULL until first sync
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT clusters_status_check
        CHECK (status IN ('connected', 'disconnected')),

    CONSTRAINT clusters_lookback_window_check
        CHECK (lookback_window IN ('7d', '10d', '30d'))
);
