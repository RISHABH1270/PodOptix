-- Migration 001: Create clusters table
-- Stores every Kubernetes cluster registered in PodOptix

CREATE TABLE IF NOT EXISTS clusters (
    id             VARCHAR(36)  PRIMARY KEY,                -- UUID
    name           VARCHAR(255) NOT NULL UNIQUE,            -- human-readable name
    prometheus_url VARCHAR(500) NOT NULL,                   -- Prometheus HTTP endpoint
    token          TEXT         NOT NULL,                   -- encrypted auth token
    window         VARCHAR(10)  NOT NULL DEFAULT '7d',      -- how far back to query e.g. "7d"
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
