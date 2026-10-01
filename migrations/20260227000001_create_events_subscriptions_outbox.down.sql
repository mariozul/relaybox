DROP INDEX IF EXISTS idx_outbox_pending;
DROP INDEX IF EXISTS idx_subscriptions_tenant;
DROP INDEX IF EXISTS idx_events_tenant_type;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS events;
