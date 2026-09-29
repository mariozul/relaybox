# Relaybox architecture RFC

Status: PROPOSED — awaiting Human Gate 1. Approval of the Stage 1 authoring plan did not approve this RFC.
Requirements: `specs/reqs/webhooks-relaybox.md` (all original FR/AC IDs retained).
Scope: specification only; no application code, executable tests, migrations or dependency changes.

## Context and impact
The current Go 1.23 scaffold has process health routes and lifecycle wiring in `cmd/relaybox/main.go`; readiness currently does not check PostgreSQL. No business interfaces are assumed to exist.
New persistence, public endpoints and external HTTP delivery make blast radius HIGH and reqspec triage REQUIRED.

## Seven-layer boundaries
| Layer | Proposed ownership | Contract |
| --- | --- | --- |
| Entity | pkg/domain | Event, Subscription, Delivery, identity and typed errors; no transport/driver imports |
| Domain service | pkg/domain | Delivery policy and lifecycle invariants; injected clock |
| Use case / ports | pkg/service | Ingest, create/list/delete subscriptions; context-first contracts |
| Repository interface | pkg/repository | Tenant-scoped persistence; reference domain types and inward-defined I/O abstractions |
| Infrastructure | pkg/infrastructure | PostgreSQL transactions, claim persistence, HTTP delivery and telemetry adapters |
| Transport | pkg/handler | Auth-context extraction, request validation, error/status mapping, probes |
| DTO / mapper | pkg/handler | Explicit request/response conversion without leaking persistence types |
Composition belongs in cmd/relaybox; background orchestration uses bounded context-owned workers. Concrete method signatures and target files follow locked testspec, not this RFC.

## Identity and isolation
All APIs require an authenticated tenant/actor except deployment health/metrics routes. Missing verified identity fails closed. The deployment must select the authentication verifier before Stage 2; unsigned client headers are not a verifier.
Every tenant-owned lookup, mutation, join and uniqueness boundary includes tenant identity. Outbox rows store tenant identity and enforce event/subscription tenant consistency. A dispatcher uses the persisted identity from trusted ingestion, never a caller-controlled query field.
Cross-tenant deletion returns the same 404 as a missing subscription. List is tenant-scoped and paginated (proposed default 50, maximum 100).

## Durable event and fan-out boundary
One transaction persists the event and deliveries for active subscriptions visible in its ingestion snapshot. Uniqueness is tenant/dedup key for events and event/subscription for deliveries; concurrent repeats return the committed original event ID without re-fan-out.
Snapshot each matching target URL and a stable delivery ID. Store trace propagation metadata separately from arbitrary event payload and exclude it from logs. An event without matches commits with zero deliveries; adding subscriptions later does not replay history.
Subscription deletion is soft deletion. It affects future matching, not existing deliveries; queued deliveries continue to their snapshotted targets. Approving this RFC explicitly accepts that behavior.
An HTTP response may be lost after successful ingestion; callers retry with the same dedup key. Same key with different content returns the original ID without replacing the original event.

## Delivery state and recovery
Durable states are pending, claimed, delivered and deadletter. Claims have an owner token and expiry; completion requires current claim ownership so stale workers cannot overwrite newer outcomes.
Acquire/reclaim bounded batches in short transactions and commit before HTTP I/O. Persist retry schedule and attempts. Expired claims become eligible after a crash; committed pending work survives restart.
Successful HTTP receipt followed by a crash before acknowledgment can cause duplicate POSTs. Stable delivery ID lets subscribers deduplicate; the service does not claim exactly-once delivery.
Proposed defaults: 8 workers, claim batch at most 8, HTTP timeout 10 seconds, claim lease 30 seconds, shutdown grace 15 seconds, permanent-failure limit N=5. All are validated positive configuration; lease must exceed HTTP timeout plus persistence margin. Slow/stale claim handling must remain safe under lease expiry.
Backoff starts at 1 second, doubles to a 5-minute cap and uses injected clock scheduling. Transient failures retry indefinitely at the cap; permanent failures dead-letter when total completed attempts reaches N. A crash before completion may resend without incrementing the durable completed-attempt count.
2xx is success; timeout/connection failures and 5xx are transient; all 4xx including 429 follow the TRD permanent-failure policy. No automatic redirects; proposed 3xx handling is permanent failure after N attempts. These classifications require explicit approval, especially 429.
Workers inherit service cancellation, propagate ctx into SQL and HTTP, and stop claiming during shutdown. Bind cleanup immediately after resource acquisition. Cancellation cannot discard durable pending work.

## Outbound URL security decision
Propose HTTPS-only URLs without userinfo, rejecting loopback/private/link-local/metadata destinations for public subscriber delivery. Enforce the destination restriction on resolved connection addresses, not just at subscription creation; disable redirects to avoid bypass.
This is an explicit proposed security addition, not a claim that the original TRD already specifies an SSRF policy. Deployment egress controls remain defense in depth. Approve this policy or supply the intended private-endpoint allowance before downstream spec lock.

## Persistence and migration intent
Events, subscriptions and outbox all carry creation/update/deletion actors and UTC timestamps; background writes use an identifiable service actor. Deletion metadata is nullable until deletion.
Proposed migration pair: `migrations/20260929000100_relaybox.up.sql` and `migrations/20260929000100_relaybox.down.sql`. These paths describe future work, not files created in Stage 1.
Expand adds tables, tenant-safe references, uniqueness and indexes for matching, pagination and due-delivery claims. No existing domain records require backfill. No destructive contract phase is needed for this first schema.
Down migration requires stopping delivery/ingestion and backing up data; dropping these tables destroys events and pending work and is never an automatic production rollback. Prefer rolling back application activation while retaining schema/data.

## Observability and API decisions
POST /v1/events returns 201 for new events and 200 with the original ID for repeats. Subscription creation returns 201, list returns 200, deletion at /v1/subscriptions/{id} returns 204; repeated deletion returns 404. No PUT/PATCH endpoint is implied by the TRD word CRUD.
Define request duration histograms and error counters with normalized route/method/status-class labels. Delivery outcomes use only delivered, failed and deadletter values. Never label by tenant, URL, event type, payload or IDs.
Emit context-bound JSON with trace/span IDs; do not log payloads, dedup keys, credential-bearing URLs or auth material. Hot paths avoid success/info verbosity. Propagate trace metadata on outbound HTTP and SQL; link recovered delivery spans to persisted ingestion context while cancellation follows worker lifecycle.
/livez measures process life; /readyz performs bounded DB reachability checks and returns 503 on failure. Metrics exposition must follow deployment access restrictions.

## Verification obligations and follow-on gate
After Human Gate 1, formulate categorized deterministic tests with explicit violation signals for every atomic requirement and applicable invariant. Lock testspec before specifying concrete interfaces, file-level DAG and implementation slices.
Include rollback, concurrent dedup, restart/lease recovery, tenant isolation, stable IDs, 500→200 and 400 retry schedules, body cleanup, shutdown/races, trace propagation, telemetry cardinality/redaction and readiness failures.
Future verification uses scoped race-enabled tests, injected time and synchronization without sleeps, fixture cleanup, at least 85% domain/service statement coverage and complete sentinel branches.
The complete spec bundle must pass canonical JSON-schema validation, Gate B and semantic RTM audit. This partial Gate 1 checkpoint cannot pass Gate B because testspec, implspec and execution manifest intentionally do not yet exist.

## Approval requested
Approve the architecture, membership/deletion semantics, authentication integration boundary, configurable defaults, 429/3xx classification and outbound URL restrictions above, or provide amendments. Approval unlocks testspec authoring, not implementation.
