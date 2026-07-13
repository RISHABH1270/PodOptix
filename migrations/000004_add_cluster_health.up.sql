-- Migration 004: Add status and last_synced_at to clusters table

ALTER TABLE clusters
    ADD COLUMN status           VARCHAR(20)  NOT NULL DEFAULT 'healthy',
    ADD COLUMN last_synced_at TIMESTAMPTZ;
