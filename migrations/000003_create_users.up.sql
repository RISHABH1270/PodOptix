-- Migration 003: Create users table
-- Stores PodOptix dashboard users.
-- Passwords stored as bcrypt hashes — never plain text.

CREATE TABLE IF NOT EXISTS users (
    user_id       VARCHAR(36)  PRIMARY KEY,                -- UUID
    email         VARCHAR(255) NOT NULL UNIQUE,            -- login identifier
    password_hash TEXT         NOT NULL,                   -- bcrypt hash
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
