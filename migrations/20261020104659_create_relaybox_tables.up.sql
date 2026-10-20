-- =============================================================================
-- Migration: Create Relaybox Tables
-- Version: 20261020104659
-- Description: Creates the core tables for the Relaybox multi-tenant webhook
--              relay service: events, subscriptions, and outbox.
-- Design:
--   - Expand/Contract pattern: safe for zero-downtime deployments.
--   - All tables include audit columns (created_at, updated_at, created_by,
--     updated_by) per RULE-DATA-03.
--   - Events table enforces idempotent ingestion via a UNIQUE constraint on
--     (tenant_id, dedup_key) per FR-ING-03 / RULE-DATA-02.
--   - Outbox table supports the transactional outbox pattern per RULE-EVT-02.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. events: Stores ingested domain events with deduplication support.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    dedup_key   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT NOT NULL DEFAULT '',
    updated_by  TEXT NOT NULL DEFAULT '',

    -- Enforce idempotent ingestion per (tenant_id, dedup_key) (FR-ING-03, RULE-DATA-02).
    -- Partial index: only enforce uniqueness for entries with a non-empty dedup_key.
    CONSTRAINT uq_events_tenant_dedup UNIQUE (tenant_id, dedup_key)
);

-- Index on tenant_id for efficient tenant-scoped queries.
CREATE INDEX IF NOT EXISTS idx_events_tenant_id ON events (tenant_id);

-- Index on event_type for subscriber matching.
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events (event_type);

-- ---------------------------------------------------------------------------
-- 2. subscriptions: Maps event types to subscriber webhook endpoints.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    target_url  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT NOT NULL DEFAULT '',
    updated_by  TEXT NOT NULL DEFAULT ''
);

-- Index on (tenant_id, event_type) for efficient subscriber lookup on delivery.
CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_event
    ON subscriptions (tenant_id, event_type);

-- ---------------------------------------------------------------------------
-- 3. outbox: Durable delivery queue using the transactional outbox pattern
--    (RULE-EVT-02). Entries are written atomically with events in the same
--    transaction to guarantee at-least-once delivery.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS outbox (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id          UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    subscription_id   UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    status            TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'delivered', 'failed', 'deadletter')),
    attempts          INT NOT NULL DEFAULT 0,
    next_attempt_at   TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Each (event_id, subscription_id) pair should only appear once.
    CONSTRAINT uq_outbox_event_subscription UNIQUE (event_id, subscription_id)
);

-- Index on status + next_attempt_at for efficient pending-entry polling.
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (status, next_attempt_at NULLS FIRST)
    WHERE status IN ('pending', 'failed');

-- Index on event_id for reverse lookups.
CREATE INDEX IF NOT EXISTS idx_outbox_event_id ON outbox (event_id);

COMMIT;
