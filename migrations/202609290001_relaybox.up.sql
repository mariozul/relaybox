CREATE TABLE events (
    id text PRIMARY KEY,
    tenant_id text NOT NULL CHECK (tenant_id <> ''),
    event_type text NOT NULL CHECK (event_type <> ''),
    payload jsonb NOT NULL,
    dedup_key text NOT NULL CHECK (dedup_key <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    created_by text NOT NULL CHECK (created_by <> ''),
    updated_by text NOT NULL CHECK (updated_by <> ''),
    deleted_by text,
    CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)),
    CHECK (deleted_by IS NULL OR deleted_by <> ''),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, dedup_key)
);
CREATE TABLE subscriptions (
    id text PRIMARY KEY,
    tenant_id text NOT NULL CHECK (tenant_id <> ''),
    event_type text NOT NULL CHECK (event_type <> ''),
    target_url text NOT NULL CHECK (target_url <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    created_by text NOT NULL CHECK (created_by <> ''),
    updated_by text NOT NULL CHECK (updated_by <> ''),
    deleted_by text,
    CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)),
    CHECK (deleted_by IS NULL OR deleted_by <> ''),
    UNIQUE (tenant_id, id)
);
CREATE INDEX subscriptions_matching ON subscriptions (tenant_id, event_type) WHERE deleted_at IS NULL;
CREATE TABLE outbox (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    event_id text NOT NULL,
    subscription_id text NOT NULL,
    target_url text NOT NULL CHECK (target_url <> ''),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','delivered','deadletter')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    lease_token text,
    lease_until timestamptz,
    trace_parent text NOT NULL DEFAULT '',
    trace_state text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    created_by text NOT NULL CHECK (created_by <> ''),
    updated_by text NOT NULL CHECK (updated_by <> ''),
    deleted_by text,
    CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)),
    CHECK (deleted_by IS NULL OR deleted_by <> ''),
    CHECK ((status = 'processing') = (lease_token IS NOT NULL AND lease_until IS NOT NULL)),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
    UNIQUE (event_id, subscription_id),
    FOREIGN KEY (tenant_id, event_id) REFERENCES events (tenant_id, id),
    FOREIGN KEY (tenant_id, subscription_id) REFERENCES subscriptions (tenant_id, id)
);
CREATE INDEX outbox_due ON outbox (next_attempt_at, id) WHERE status = 'pending';
CREATE INDEX outbox_expired ON outbox (lease_until, id) WHERE status = 'processing';
