CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload BYTEA NOT NULL,
    dedup_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT NOT NULL DEFAULT 'system',
    CONSTRAINT uq_events_tenant_dedup UNIQUE (tenant_id, dedup_key)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    target_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT NOT NULL DEFAULT 'system',
    updated_at TIMESTAMPTZ,
    updated_by TEXT,
    deleted_at TIMESTAMPTZ,
    deleted_by TEXT
);

CREATE TABLE IF NOT EXISTS outbox (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id),
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id),
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_attempt_at TIMESTAMPTZ,
    delivery_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT NOT NULL DEFAULT 'system'
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(status, next_attempt_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant ON subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_events_tenant_type ON events(tenant_id, event_type);
