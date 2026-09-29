# Relaybox ImplSpec

## Contracts and layers
Pure pkg/domain defines Principal{TenantID,ActorID}, Audit{CreatedAt,UpdatedAt,DeletedAt,CreatedBy,UpdatedBy,DeletedBy}, Event{ID,TenantID,EventType,Payload,DedupKey,Audit}, Subscription{ID,TenantID,EventType,TargetURL,Audit}, Delivery{ID,TenantID,EventID,SubscriptionID,TargetURL,Payload,TraceParent,TraceState,State,Attempts,LeaseToken,LeaseUntil,NextAttemptAt,Audit}. Payload is []byte; tracing fields are strings, not SDK types.
Clock.Now() time.Time; State.CanTransition(next State) bool; sentinel errors ErrInvalid, ErrUnauthorized, ErrNotFound, ErrConflict, ErrUnavailable.
EventStore.Ingest(ctx context.Context, principal Principal, event Event) (Event,error): one transaction persists tenant-deduplicated event plus all active matching subscription snapshots; no remote I/O; repeat returns original event.
SubscriptionStore.Create(ctx context.Context, principal Principal, subscription Subscription) (Subscription,error); List(ctx context.Context, principal Principal, after string, limit int) ([]Subscription,error); Delete(ctx context.Context, principal Principal, id string) error.
OutboxStore.Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Delivery,error); Complete(ctx context.Context, delivery Delivery, state State, next time.Time) error. Claims have expiring fenced ownership; stale tokens return ErrConflict. All DB work propagates context.
Sender.Send(ctx context.Context, delivery Delivery) (int,error); bounded HTTP attempt, public HTTPS only, connect-time IP filtering, no redirects, immediate response cleanup, stable X-Relaybox-Delivery-Id and OTel propagation. Persistence happens outside HTTP transaction lifetime.
Verifier.Verify(ctx context.Context, requestToken string) (Principal,error): adapter supplied by verified gateway contract; no unsigned tenant-header fallback.

## Seven-layer implementation ownership
Migrations → pkg/domain contracts → pkg/repository PostgreSQL → pkg/service use cases → pkg/handler DTOs → pkg/worker dispatcher → cmd/relaybox wiring. pkg/infrastructure implements HTTP and telemetry ports. Imports point inward; domain never imports SQL/HTTP/SDKs. Third-party errors translate to sentinel errors; logs never include payload/credentials/DSN.

## Persistence and lifecycle
migrations/202609290001_relaybox.up.sql and .down.sql introduce events/subscriptions/outbox, audit metadata, unique tenant/dedup and event/subscription pairs, tenant-consistent references and due-work indexes. Additive expansion; no legacy backfill needed. Down permitted only on empty deployment, never erase accepted work for rollback.
States pending→processing→delivered/deadletter/pending. Terminal states immutable. Expired processing claims can be fenced and reclaimed without changing delivery identity. Attempts increment durably at claim. Snapshot deletion never deletes queued deliveries.
2xx success; 4xx deadletter at configurable N=5 total attempts; timeouts/network/5xx/unexpected 3xx retry indefinitely. Exponential jitter bounded by 1 second base/5 minute cap; default attempt timeout 10 seconds; lease exceeds attempt timeout plus completion budget. Inject clock/random source for deterministic tests.
Workers and queue bounded; claim no more than available capacity; root lifecycle cancellation reaches queries and HTTP; shutdown joins workers and lets unfinished leases recover. No detached dispatcher context.

## Transport, security and observability
POST /v1/events, POST/GET /v1/subscriptions, DELETE /v1/subscriptions/{id}; separate DTOs, size limits, bounded pagination and fail-closed principal validation. Error mappings are in ReqSpec; cross-tenant misses return 404. Gateway configuration is required before enabling public routes.
/livez independent of DB; /readyz deadline-bounded DB check returns 503 on failure. /metrics exposes request duration histogram/error counter and delivery outcomes with status delivered|failed|deadletter. Labels only route template, bounded method/status; never tenant/URL/event IDs.
Persist producer traceparent/tracestate with outbox, link worker lifecycle span to producer span, inject outbound headers; instrument DB calls. Context-bound JSON logs extract trace_id/span_id. Telemetry packages are outer adapters only.

## Verification and rollback
Run Gate B and JSON schema validation before TDD. Focused race tests, formatting/lint and domain/service coverage ≥85%; every sentinel branch tested. Rollout schema then dormant service then canary workers; rollback stops dispatch but retains durable work. Vet and pin new dependencies, pause for human dependency review. No workflow edits.

## Requirement → test → implementation → task
- FR-ING-01 / AC-07 → TC-01-INT → pkg/handler/events.go → ST-06.
- FR-ING-02 / AC-02 → TC-02-INT → pkg/repository/ingestion.go → ST-04.
- FR-ING-03 / AC-01 → TC-03-RACE → pkg/repository/ingestion.go → ST-04.
- FR-SUB-01 / AC-08 → TC-04-INT → pkg/repository/subscriptions.go → ST-03.
- FR-SUB-02 / AC-04 → TC-05-INT → pkg/repository/subscriptions.go → ST-03.
- FR-DEL-01 / AC-09 → TC-06-INT → pkg/infrastructure/sender.go → ST-07.
- FR-DEL-02 / AC-03 → TC-07-UNIT → pkg/domain/policy.go → ST-08.
- FR-DEL-02 / AC-03 → TC-08-EDGE → pkg/repository/outbox.go → ST-05.
- FR-DEL-03 / AC-05 → TC-09-RACE → pkg/worker/dispatcher.go → ST-08.
- FR-DEL-04 / AC-10 → TC-10-INT → pkg/infrastructure/sender.go → ST-07.
- FR-OBS-01 / AC-06 → TC-11-INT → pkg/handler/health.go → ST-09.
- FR-OBS-02 / AC-11 → TC-12-INT → pkg/infrastructure/telemetry.go → ST-09.
- FR-DEL-02 / AC-03 → TC-13-UNIT → pkg/domain/delivery.go → ST-01.
- FR-ING-02 / AC-02 → TC-14-INT → migrations/202609290001_relaybox.up.sql → ST-02.
- FR-ING-02 FR-DEL-03 / AC-02 AC-05 → TC-15-INT → cmd/relaybox/main.go → ST-10.

## Approved toolchain amendment
The requester approved Go 1.25 in place of the original Go 1.23 constraint so a patched PostgreSQL driver can be used. The original TRD is retained unchanged for provenance. Runtime dependency selection and vetting remain separate. CI toolchain and lint compatibility must be aligned before claiming the upgrade is CI-ready; workflow changes require explicit authorization.
