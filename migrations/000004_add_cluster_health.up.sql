-- Migration 004: Add status and last_collected_at to clusters table

ALTER TABLE clusters
    ADD COLUMN status           VARCHAR(20)  NOT NULL DEFAULT 'healthy',
    ADD COLUMN last_collected_at TIMESTAMPTZ;
