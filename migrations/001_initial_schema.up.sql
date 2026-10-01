-- Relaybox v0.1.0 initial schema
-- Tables: events, subscriptions, outbox
-- Enforces: RULE-DATA-01 (atomic tx), RULE-DATA-02 (idempotency via unique),
--           RULE-DATA-03 (audit columns)

CREATE TABLE IF NOT EXISTS events (
    id          VARCHAR(64) PRIMARY KEY,
    tenant_id   VARCHAR(128) NOT NULL,
    event_type  VARCHAR(256) NOT NULL,
    payload     JSONB        NOT NULL,
    dedup_key   VARCHAR(256) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by  VARCHAR(128) NOT NULL DEFAULT 'system'
);

-- Idempotency: one event per tenant per dedup_key (FR-ING-03, RULE-DATA-02)
CREATE UNIQUE INDEX IF NOT EXISTS idx_events_tenant_dedup
    ON events (tenant_id, dedup_key);

CREATE TABLE IF NOT EXISTS subscriptions (
    id          VARCHAR(64) PRIMARY KEY,
    tenant_id   VARCHAR(128) NOT NULL,
    event_type  VARCHAR(256) NOT NULL,
    target_url  TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by  VARCHAR(128) NOT NULL DEFAULT 'api',
    updated_at  TIMESTAMPTZ,
    updated_by  VARCHAR(128),
    deleted_at  TIMESTAMPTZ,
    deleted_by  VARCHAR(128)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_event
    ON subscriptions (tenant_id, event_type) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS outbox (
    id               VARCHAR(64) PRIMARY KEY,
    event_id         VARCHAR(64)  NOT NULL REFERENCES events(id),
    subscription_id  VARCHAR(64)  NOT NULL REFERENCES subscriptions(id),
    status           VARCHAR(32)  NOT NULL DEFAULT 'pending',
    attempts         INT          NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_error       TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox (status, next_attempt_at) WHERE status = 'pending';
