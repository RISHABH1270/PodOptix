-- Migration 001: Create clusters table

CREATE TABLE IF NOT EXISTS clusters (
    cluster_id      VARCHAR(36)  PRIMARY KEY,                          -- UUID
    name            VARCHAR(255) NOT NULL UNIQUE,                      -- human-readable name
    prometheus_url  VARCHAR(500) NOT NULL,                             -- Prometheus HTTP endpoint
    token           TEXT         NOT NULL,                             -- AES-256-GCM encrypted auth token
    lookback_window VARCHAR(10)  NOT NULL DEFAULT '7d',                -- how far back to query e.g. "7d"
    status          VARCHAR(20)  NOT NULL DEFAULT 'healthy',           -- healthy | unhealthy
    last_synced_at  TIMESTAMPTZ,                                       -- NULL if never collected
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
