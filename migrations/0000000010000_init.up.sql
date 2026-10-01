-- Relaybox initial schema: events, subscriptions, outbox.
-- All tables include audit columns per RULE-DATA-03.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Events ingested for relay.
CREATE TABLE IF NOT EXISTS events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  TEXT        NOT NULL,
    event_type TEXT        NOT NULL,
    payload    BYTEA       NOT NULL,
    dedup_key  TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT        NOT NULL DEFAULT 'system',

    CONSTRAINT uq_events_tenant_dedup UNIQUE (tenant_id, dedup_key)
);

-- Registered subscriber endpoints, scoped per tenant.
CREATE TABLE IF NOT EXISTS subscriptions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  TEXT        NOT NULL,
    event_type TEXT        NOT NULL,
    target_url TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT        NOT NULL DEFAULT 'system'
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_event
    ON subscriptions (tenant_id, event_type);

-- Transactional outbox for at-least-once delivery (RULE-EVT-02).
CREATE TABLE IF NOT EXISTS outbox (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id         UUID        NOT NULL REFERENCES events(id),
    subscription_id  UUID        NOT NULL REFERENCES subscriptions(id),
    status           TEXT        NOT NULL DEFAULT 'pending',
    attempts         INT         NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error       TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (status, next_attempt_at)
    WHERE status = 'pending';
