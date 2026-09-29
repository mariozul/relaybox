# Relaybox ImplSpec

Architecture and TestSpec approved; this document defines contracts, not executable implementations. ReqSpec and TestSpec with the same date are normative. Stage 2 is a separate approval/delivery cycle.

## Seven layers and dependency direction
1. migrations: reversible PostgreSQL DDL.
2. pkg/domain: pure entities, errors and all I/O ports; standard-library context/time only.
3. pkg/infrastructure/postgres: persistence adapters implement inward domain ports.
4. pkg/service: pure usecases and retry policy depend on domain ports.
5. pkg/handler: transport DTOs, mapping, authentication and health.
6. pkg/worker: bounded dispatcher and durable recovery.
7. pkg/app and cmd/relaybox: dependency injection, signals and shutdown.
Telemetry and HTTP delivery are infrastructure adapters, never domain imports. DTOs never double as entities. No context.Background/TODO inside services or dispatcher; root context belongs only to composition.

## Domain contracts
```go
type Identity struct { TenantID string; ActorID string }
type Audit struct { CreatedAt, UpdatedAt time.Time; DeletedAt *time.Time; CreatedBy, UpdatedBy string; DeletedBy *string }
type Event struct { ID, TenantID, EventType, DedupKey string; Payload []byte; Audit Audit }
type Subscription struct { ID, TenantID, EventType, TargetURL string; Audit Audit }
type Delivery struct { ID, TenantID, EventID, SubscriptionID, TargetURL, Status, ClaimToken string; Payload []byte; TraceParent, TraceState string; Attempts int; NextAttemptAt, LeaseUntil time.Time; Audit Audit }
type IngestInput struct { EventType, DedupKey string; Payload []byte }
type IngestResult struct { EventID string; Created bool }
type SubscriptionInput struct { EventType, TargetURL string }
type Page struct { Limit int; Cursor string }
type SubscriptionPage struct { Items []Subscription; NextCursor string }
type Outcome struct { Kind string; CompletedAt, NextAttemptAt time.Time }
type Clock interface { Now() time.Time }
type Events interface { Ingest(context.Context, IngestInput) (IngestResult, error) }
type Subscriptions interface {
 Create(context.Context, SubscriptionInput) (Subscription, error)
 List(context.Context, Page) (SubscriptionPage, error)
 Delete(context.Context, string) error
}
type Outbox interface {
 Claim(context.Context, int, time.Duration) ([]Delivery, error)
 Complete(context.Context, Delivery, Outcome) error
}
type Sender interface { Send(context.Context, Delivery) (int, error) }
type Health interface { Ping(context.Context) error }
func WithIdentity(context.Context, Identity) context.Context
func IdentityFrom(context.Context) (Identity, error)
```
Identity context keys are private domain types, set only after gateway validation or from trusted persisted worker identity. Ports reject missing identity. Sentinel errors: ErrUnauthenticated (401), ErrInvalidInput (400), ErrNotFound (404), ErrConflict (409), ErrUnavailable (503), ErrInternal (500). Adapter wraps driver errors internally and returns classified domain errors; transport never leaks raw causes. Cancellation propagates unchanged and does not invent a client response.

## Authentication and HTTP
Gateway middleware requires one matching X-Internal-Secret and one valid UUID X-Tenant-ID. Empty GATEWAY_SHARED_SECRET fails startup. Compare fixed-length secret digests in constant time. Missing/invalid/duplicate credentials return 401 before service invocation. Optional X-Actor-ID is audit metadata; absent actor uses gateway-system. Never propagate these three headers to subscriber HTTP calls.
POST /v1/events accepts event_type, payload, dedup_key; 201 new and 200 replay return event_id. Unknown identity fields cannot influence ownership. Invalid/malformed JSON or missing event fields returns 400. POST /v1/subscriptions accepts event_type,target_url, returns 201; GET returns 200 with default limit 50, maximum 100 and opaque cursor; DELETE /v1/subscriptions/{id} returns 204, missing/inaccessible/deleted returns 404. No update endpoint. Probe/metrics routes are not business authentication endpoints and require deployment network restrictions.

## Persistence contracts and migrations
migrations/20260929000100_relaybox.up.sql creates subscriptions, events and outbox; matching down removes outbox before its parents. All rows carry tenant_id and UTC created_at/updated_at/deleted_at plus created_by/updated_by/deleted_by; deletion fields nullable. Service background audit actor is relaybox-dispatcher.
Events unique (tenant_id,dedup_key); events and subscriptions expose unique (tenant_id,id) keys. Outbox tenant-safe composite foreign keys reference both, and unique (tenant_id,event_id,subscription_id) prevents repeated fanout. Outbox stores target URL and payload reference, trace metadata, stable id, status, attempts, next_attempt_at, claim_token and lease_until. Index active tenant/type subscriptions and due/expired deliveries.
Ingest persists event and snapshot fanout in one transaction, binds rollback immediately, and commits before network I/O. Concurrent dedup returns original event ID without fanout or payload replacement. No subscription matches is a valid persisted event. Later subscription creation is not retroactive; soft deletion does not cancel existing deliveries.
Claim is a short transactional bounded lease acquisition; multiple dispatchers cannot own the same unexpired token. Completion compares token and tenant; stale owners cannot overwrite newer claims. Crash before claim commit or after HTTP receipt is recoverable; exact-once external effects are not promised. Lease expiry allows duplicate delivery with the original delivery ID.
Expand: create additive tables/indexes while service feature remains inactive. Migrate: no existing data to backfill. Contract: no destructive follow-up needed. Production rollback disables application feature and retains data. Destructive down is only for disposable fixtures or explicitly authorized, backed-up, quiesced environments.

## Delivery and lifecycle
Default W=8 workers, claim batch <= available capacity, HTTP timeout 10s, lease 30s, shutdown grace 15s. Validate positive configuration and lease greater than timeout plus persistence margin. No unbounded goroutines or per-request background work. Worker cancellation stops new claims and propagates to every SQL/HTTP call; shutdown waits within grace, leaving unacknowledged work recoverable.
Each attempt uses outbox ID as X-Relaybox-Delivery-Id. A 2xx marks delivered. Timeout/connection/5xx retries indefinitely with exponential backoff from 1s capped at 5m. All 4xx, including 429, and non-followed 3xx deadletter once total completed attempts reaches configurable N (default 5). Interrupted uncompleted attempts may resend without incrementing completed count. Persist schedule and attempts atomically on completion; never hold a database transaction across HTTP. Bind response/row/transaction cleanup immediately; close each HTTP body inside the attempt scope, not at worker exit.
HTTPS-only subscription URLs, no userinfo, no redirects; deny loopback/private/link-local/metadata destinations at validation and resolved connection time. Resolution/dial preserve context; validate every candidate IP to prevent DNS rebinding. Tests use injected safe dial fixtures without production bypass flags.

## Telemetry
Persist ingestion traceparent/tracestate outside payload; recovered spans link to this parent while cancellation stays attached to worker lifetime. Inject OpenTelemetry headers on HTTP; SQL spans inherit context. Context-bound slog JSON contains trace_id/span_id/component and sanitized classification, not payloads, secrets, dedup keys or credential URLs.
Prometheus request_duration_seconds histogram buckets .005,.01,.025,.05,.1,.25,.5,1,2.5,5,10; normalized route, bounded method, status class only. Request error counters and business mutation counters use finite operations/status. delivery_outcomes_total has only status=delivered|failed|deadletter. Unknown route/method collapses to a fixed sentinel; no tenant, URL, ID or event-type labels. /readyz performs deadline-bound DB ping (503 on failure), /livez stays independent of DB.

## Dependencies and verification
Current module has only Go standard library. Stage 2 dependency foundation must vet and pin a maintained PostgreSQL driver, OpenTelemetry SDK/propagation and Prometheus client, checking permissive licenses, release activity and advisories; seek human review before adding them. No dependencies are added in this specification PR. Stdlib net/http, slog, crypto/subtle and UUID validation avoid extra framework/auth packages.
Use table-driven tests, injected clocks, barriers, fixture cleanup and focused race commands from manifest; never the entire suite locally. TestSpec controls >=85% domain/service coverage and 100% sentinel-branch proof. Every slice must compile and its focused tests pass; adapter tests may construct fixtures from already-merged domain contracts. Below estimates include tests and migrations; split before exceeding 500 LOC, preserving explicit file collision ordering. Test IDs below resolve to the TestSpec FR/REQ/AC and invariant RTM.

## Implementation ownership / DAG
| Slice | Layer | LOC | Depends on | Tests |
|---|---|---:|---|---|
| ST-01 Domain contracts | domain | 250 | none | TC-11-UNIT |
| ST-02 Dependency and configuration foundation | config | 220 | ST-01 | TC-01-INT |
| ST-03 Reversible schema | repository | 250 | ST-02 | TC-12-INT |
| ST-04 Subscription persistence | repository | 350 | ST-03 | TC-04-INT |
| ST-05 Atomic ingestion | repository | 420 | ST-04 | TC-02-INT, TC-03-RACE |
| ST-06 Claim and completion persistence | repository | 380 | ST-05 | TC-07-RACE, TC-08-INT |
| ST-07 Usecases and retry policy | service | 350 | ST-01 | TC-06-EDGE, TC-11-UNIT |
| ST-08 Trace logs and metrics | adapter | 350 | ST-02 | TC-10-UNIT |
| ST-09 Safe outbound HTTP | adapter | 400 | ST-08 | TC-05-INT, TC-14-EDGE |
| ST-10 Gateway middleware | transport | 300 | ST-02 | TC-01-INT |
| ST-11 API and health | transport | 420 | ST-07, ST-08, ST-10 | TC-09-INT, TC-13-INT |
| ST-12 Bounded dispatcher | service | 400 | ST-06, ST-07, ST-09 | TC-07-RACE, TC-08-INT |
| ST-13 Composition and shutdown | config | 300 | ST-11, ST-12 | TC-09-INT, TC-07-RACE |
| ST-14 Crash recovery integration | test | 350 | ST-13 | TC-15-INT |

Exact target-file whitelists and focused commands are authoritative in the accompanying execution JSON. Shared go.mod/go.sum ownership belongs to ST-02; later dependency changes require a serialized manifest amendment. ST-06 persistence obligations for TC-07-RACE/TC-08-INT are integration-verified when ST-12 worker fixtures exist; its own adapter tests must independently prove leases/fencing before merge.
