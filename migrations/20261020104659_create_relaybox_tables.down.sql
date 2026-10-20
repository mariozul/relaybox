-- =============================================================================
-- Migration Rollback: Drop Relaybox Tables
-- Version: 20261020104659
-- Description: Reverses the up migration by dropping the outbox, subscriptions,
--              and events tables in dependency order (child tables first).
-- Design:
--   - Drops indexes before tables for cleaner rollback.
--   - Uses IF EXISTS for idempotent re-execution.
-- =============================================================================

BEGIN;

-- Drop outbox indexes and table first (child table).
DROP INDEX IF EXISTS idx_outbox_event_id;
DROP INDEX IF EXISTS idx_outbox_pending;
DROP TABLE IF EXISTS outbox;

-- Drop subscriptions indexes and table.
DROP INDEX IF EXISTS idx_subscriptions_tenant_event;
DROP TABLE IF EXISTS subscriptions;

-- Drop events indexes and table last (parent table).
DROP INDEX IF EXISTS idx_events_event_type;
DROP INDEX IF EXISTS idx_events_tenant_id;
DROP TABLE IF EXISTS events;

COMMIT;
