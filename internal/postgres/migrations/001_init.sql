CREATE TABLE IF NOT EXISTS events (
 id uuid PRIMARY KEY, tenant_id text NOT NULL, event_type text NOT NULL,
 payload jsonb NOT NULL, dedup_key text NOT NULL, created_at timestamptz NOT NULL,
 created_by text NOT NULL, UNIQUE (tenant_id, dedup_key), UNIQUE (tenant_id, id)
);
CREATE TABLE IF NOT EXISTS subscriptions (
 id uuid PRIMARY KEY, tenant_id text NOT NULL, event_type text NOT NULL,
 target_url text NOT NULL, created_at timestamptz NOT NULL, UNIQUE (tenant_id, id)
);
CREATE TABLE IF NOT EXISTS outbox (
 id uuid PRIMARY KEY, tenant_id text NOT NULL, event_id uuid NOT NULL,
 subscription_id uuid NOT NULL, status text NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL, lease_token uuid, lease_until timestamptz,
 last_error text, delivered_at timestamptz, UNIQUE (event_id, subscription_id),
 FOREIGN KEY (tenant_id,event_id) REFERENCES events(tenant_id,id),
 FOREIGN KEY (tenant_id,subscription_id) REFERENCES subscriptions(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(next_attempt_at) WHERE status IN ('pending','inflight');
