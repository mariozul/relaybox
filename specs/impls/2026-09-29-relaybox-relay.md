# Relaybox ImplSpec and architecture decision

Approved architecture: HIGH blast radius; single relaybox domain. Follow the repository two-stage workflow: complete spec PR, review/merge, then serial implementation PRs ST-01 through ST-10. Do not modify workflows. Every PR <=500 changed lines including tests; split further before exceeding the cap. Existing spec/relaybox work remains untouched.

## Seven boundaries and contracts
1. Migrations: additive tables/indexes before enabling traffic. Up/down paths are in ST-02; destructive down migration only for empty/dev databases. Rollback application without dropping durable data.
2. Pure domain: Event, Subscription, Delivery, Identity, Claim, Outcome; errors ErrInvalid, ErrUnauthenticated, ErrNotFound, ErrConflict, ErrStaleClaim. No SQL/HTTP/telemetry imports.
3. Persistence adapters: tenant-qualified queries, mapped domain errors, context on every call; short claims and fenced updates, never network I/O under a transaction.
4. Domain services: validated identity, subscription and ingestion use cases; injected clock/random and ports.
5. HTTP DTO adapters: do not expose persistence records. Missing identity=401; invalid JSON/URL=400; oversized payload=413; foreign ID=404; unavailable DB=503. Create=201, duplicate=200, list=200, delete=204.
6. Dispatcher/outbound: bounded workers derive all calls from process ctx, durable lease recovery, immediate cleanup and bounded draining.
7. Composition: lifecycle/DI/config, DB readiness, safe shutdown, no context.Background in dispatcher or services.

```go
type Identity struct { TenantID, ActorID string }
type Clock interface { Now() time.Time }
type EventStore interface { Ingest(ctx context.Context, who Identity, input NewEvent) (Event, bool, error) }
type SubscriptionStore interface { Create(ctx context.Context, who Identity, input NewSubscription) (Subscription, error); List(ctx context.Context, who Identity) ([]Subscription, error); Delete(ctx context.Context, who Identity, id string) error }
type DeliveryStore interface { Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]Claim, error); Complete(ctx context.Context, claim Claim, result Outcome) error }
type Sender interface { Send(ctx context.Context, delivery Delivery) (Outcome, error) }
type IdentityVerifier interface { Verify(ctx context.Context, credential string) (Identity, error) }
```

## Data and protocol decisions
Events: id, tenant_id, event_type, payload, dedup_key; unique tenant/dedup. Subscriptions: id, tenant_id, event_type, target_url. Outbox: id, tenant_id, event_id, subscription_id, destination snapshot, trace context, status, attempts, next_attempt_at, lease_until, claim_token. Unique event/subscription; composite tenant-consistent references. Every table tracks created_at/by, updated_at/by, deleted_at/by in UTC; workers use explicit system actor.
Event and matching active subscription snapshots commit together. Duplicate keys return original event without fan-out, even for changed payload. Zero matches remains durable; no retroactive fan-out. Soft deletion does not destroy accepted work.
Completion requires current claim token; expired leases recover after crash. Delivery ID is outbox ID. 2xx delivered; transient network/timeout/5xx retry indefinitely with injected exponential jitter (1s initial, 5min cap). Permanent 3xx/4xx dead-letter at configurable N=3 total attempts. Cancellation must not mark success.
Public HTTPS only, no URL credentials, reject non-public DNS results at connection time and disable redirects. Tests inject local policy explicitly. 10s timeout, 8 workers, 1 MiB payload; validate bounded pool/queue and lease duration against request timeout.

## Observability and security
Persist W3C trace context; link attempt spans to ingestion and inject outbound HTTP metadata. Trace SQL with ctx. JSON slog binds trace_id, span_id, component and sanitized error; no secrets/payload/credential URLs. Request duration histogram buckets: .005,.01,.025,.05,.1,.25,.5,1,2.5,5,10 seconds. Request/error labels only method, route template and bounded status class. Delivery counter labels only delivered|failed|deadletter. DB-backed readiness deadline; liveness independent.

## Traceability
The following links identify test contracts implemented by each slice; TestSpec supplies exact FR/AC/test-file links.
- ST-01: TC-10-UNIT, TC-17-UNIT.
- ST-02: TC-02-INT, TC-04-INT, TC-07-INT, TC-18-INT.
- ST-03: TC-06-INT, TC-07-INT, TC-18-INT.
- ST-04: TC-03-EDGE, TC-08-INT, TC-12-EDGE, TC-14-INT, TC-18-INT.
- ST-05: TC-02-INT, TC-04-INT, TC-05-RACE, TC-18-INT.
- ST-06: TC-01-INT, TC-06-INT, TC-07-INT.
- ST-07: TC-08-INT, TC-09-EDGE, TC-11-INT, TC-12-EDGE, TC-14-INT.
- ST-08: TC-08-INT, TC-10-UNIT, TC-11-INT, TC-12-EDGE, TC-13-RACE, TC-14-INT.
- ST-09: TC-15-INT, TC-16-INT.
- ST-10: TC-03-EDGE, TC-13-RACE, TC-14-INT, TC-15-INT, TC-16-INT.

## Gates and prerequisites
Gateway verification contract is unresolved: do not ship trust in public headers. Driver, OTel and Prometheus packages must be vetted, pinned and separately approved before addition; prefer stdlib HTTP/slog. New package review must include maintenance, license and advisories.
TestSpec is locked before contracts. Run targeted race tests and scoped lint/format, >=85% domain-service coverage and every sentinel branch. Use real PostgreSQL fixtures; no full local suite. Follow manifest file whitelist and serial dependencies, including shared test fixtures and go.mod.
Apply additive migration before ingress/workers. Verify readiness, dedup and restart recovery before rollout. No retention, replay API, ordering, transformation or Closer/Scribe.
