# 2026-09-29 Relaybox ImplSpec
Seven layers: pkg/domain entities, pure policy services, use-case ports; repository contracts re-export domain-owned persistence ports; pkg/infrastructure concrete adapters; pkg/handler transport; DTO/mappers in handler. All dependencies point inward.
Contracts (signatures only): Ingest(ctx context.Context, identity Identity, input EventInput) (Event, error); CreateSubscription(ctx context.Context, identity Identity, input SubscriptionInput) (Subscription, error); ListSubscriptions(ctx context.Context, identity Identity) ([]Subscription, error); DeleteSubscription(ctx context.Context, identity Identity, id string) error; Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]Delivery, error); Complete(ctx context.Context, claim Claim, outcome Outcome) error; Send(ctx context.Context, delivery Delivery) (Outcome, error); Run(ctx context.Context) error; Clock.Now() time.Time.
Tenant-scoped composite keys/FKs; unique tenant/dedup and event/subscription delivery pair; all rows have created/updated/deleted actors and UTC timestamps. Outbox snapshots target URL, payload reference, trace carrier, next attempt, lease owner/token/expiry. Claim transaction ends before HTTP; completion fences stale tokens. Lease expiry recovers abandoned sends; stable outbox ID survives remote-success/local-crash duplicates.
Additive up migration plus explicit reverse-order down migration. No historical backfill on fresh tables. Never drop tables holding pending deliveries; rollback application first, drain/backup before destructive down.
See test matrix for FR/AC mapping.
ST-01 | Domain contracts | TC-12-UNIT | pkg/domain/entities.go, pkg/domain/ports.go, pkg/domain/errors.go, pkg/domain/contracts_test.go
ST-02 | Audited schema | TC-02-INT, TC-05-INT | migrations/202609290001_relaybox.up.sql, migrations/202609290001_relaybox.down.sql, pkg/infrastructure/postgres/schema_test.go
ST-03 | Subscription persistence | TC-05-INT | pkg/infrastructure/postgres/subscriptions.go, pkg/infrastructure/postgres/subscriptions_test.go, go.mod, go.sum
ST-04 | Atomic ingestion | TC-02-INT, TC-03-RACE | pkg/infrastructure/postgres/ingestion.go, pkg/infrastructure/postgres/ingestion_test.go
ST-05 | Durable claims | TC-09-INT | pkg/infrastructure/postgres/claims.go, pkg/infrastructure/postgres/claims_test.go
ST-06 | Retry policy | TC-07-EDGE | pkg/domain/retry.go, pkg/domain/retry_test.go
ST-07 | Relay use cases | TC-01-INT, TC-04-INT | pkg/service/events.go, pkg/service/subscriptions.go, pkg/service/services_test.go
ST-08 | Telemetry adapters | TC-11-INT | pkg/infrastructure/telemetry/telemetry.go, pkg/infrastructure/telemetry/telemetry_test.go, go.mod, go.sum
ST-09 | Outbound HTTP | TC-06-INT, TC-07-EDGE | pkg/infrastructure/httpdelivery/client.go, pkg/infrastructure/httpdelivery/client_test.go
ST-10 | Bounded dispatcher | TC-08-RACE, TC-09-INT | pkg/service/dispatcher.go, pkg/service/dispatcher_test.go
ST-11 | Authenticated ingress | TC-01-INT | pkg/handler/auth.go, pkg/handler/events.go, pkg/handler/events_test.go, pkg/handler/dto.go
ST-12 | Subscription routes | TC-04-INT, TC-05-INT | pkg/handler/subscriptions.go, pkg/handler/subscriptions_test.go
ST-13 | Health and wiring | TC-10-INT, TC-11-INT | pkg/handler/health.go, pkg/handler/health_test.go, cmd/relaybox/main.go, cmd/relaybox/main_test.go, go.mod
ST-14 | Recovery acceptance | TC-02-INT, TC-03-RACE, TC-08-RACE, TC-09-INT | tests/integration/recovery_test.go, tests/integration/isolation_test.go, tests/integration/load_test.go
## Confirmed gateway integration
Gateway authenticates external callers and supplies X-Tenant-ID and X-Actor-ID. Middleware validates X-Internal-Gateway-Key against GATEWAY_SHARED_KEY using constant-time comparison and places a validated Identity in context. Missing/invalid key or identity returns 401. Reject duplicate identity/key headers. Empty production configuration never bypasses verification. Local tests use explicitly injected fixtures, not an environment-triggered production fallback. Gateways must strip client-supplied identity/key headers and overwrite them; downstream transport must be encrypted and inaccessible externally.
Keep the current upstream Go 1.25 baseline; no workflow changes.
Domain ports accept explicit authenticated identity and context. All adapters translate failures to ErrInvalidInput, ErrUnauthorized, ErrNotFound, ErrConflict or ErrUnavailable. Domain Identity.Validate rejects empty, padded, control-containing or oversized identities.
