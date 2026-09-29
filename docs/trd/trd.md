# TRD: Relaybox — Multi-Tenant Webhook Relay Service

## 1. Context & Problem
Internal services need to deliver domain events to external subscriber
endpoints (customer webhooks) reliably. Direct delivery from producers couples
them to slow/unreliable third parties and risks lost events on crashes.
Relaybox is a standalone Go service that ingests events, persists them durably,
and relays them to registered subscriber endpoints with retries, idempotency,
and per-tenant isolation.

## 2. Goals
- Durable, at-least-once delivery of events to external HTTP endpoints.
- No event loss across process restarts (transactional outbox).
- Strict multi-tenant isolation.
- Production-grade observability out of the box.

## 3. Non-Goals (out of scope)
- UI/dashboard for subscribers.
- Event ordering guarantees across different event types.
- Message transformation/templating.
- The Closer & Scribe / ticket-lifecycle flow.

## 4. Stack & Architecture
- Go 1.24, 7-layer Clean Architecture (domain layer pure per RULE-ARCH-01/02).
- PostgreSQL for persistence.
- Outbound HTTP to external subscriber endpoints.
- Must satisfy all invariants in docs/rules/domain_invariants.md.

## 5. Functional Requirements

### Ingestion
- FR-ING-01: Expose `POST /v1/events` accepting a JSON event
  `{ event_type, payload, dedup_key }`. Tenant identity MUST be resolved from
  the authenticated request context, never the body (RULE-SEC-01).
- FR-ING-02: Persisting an event and enqueuing it for delivery MUST occur in a
  single database transaction (RULE-DATA-01), writing to an outbox table
  (RULE-EVT-02). No remote publish inside the open transaction.
- FR-ING-03: Ingestion MUST be idempotent per `(tenant_id, dedup_key)` via a
  unique constraint (RULE-DATA-02). Re-sends return the original event id.

### Subscriptions
- FR-SUB-01: Expose CRUD `POST/GET/DELETE /v1/subscriptions` mapping an
  `event_type` to a target URL for the authenticated tenant.
- FR-SUB-02: Subscriptions are scoped strictly to the owning tenant; a tenant
  can never read or mutate another tenant's subscriptions (RULE-SEC-01).

### Delivery (external endpoint access)
- FR-DEL-01: A background dispatcher reads pending outbox rows and sends an
  HTTP POST to each matching subscriber URL. All outbound calls MUST propagate
  `ctx` and OpenTelemetry trace metadata (RULE-RES-01, RULE-OBS-02) and set a
  timeout; `context.Background()` is forbidden in the dispatcher (RULE-RES-01).
- FR-DEL-02: Delivery is at-least-once. A 2xx response marks the row delivered;
  a transient failure (timeout, 5xx) is retried with exponential backoff; a
  permanent failure (4xx) is dead-lettered after N attempts. Response bodies
  MUST be closed immediately (RULE-RES-03).
- FR-DEL-03: The dispatcher MUST use a bounded worker pool tied to context
  cancellation and graceful shutdown; no unbounded goroutines (RULE-RES-02).
- FR-DEL-04: Delivery attempts MUST be idempotent from the subscriber's view:
  each attempt sends a stable `X-Relaybox-Delivery-Id` header (RULE-DATA-02).

### Observability & Health
- FR-OBS-01: Expose `/livez` and `/readyz`; `/readyz` checks DB reachability
  (RULE-OBS-04).
- FR-OBS-02: Emit structured JSON logs bound to ctx, and Prometheus metrics for
  request duration and delivery outcomes with bounded-cardinality labels
  (`status="delivered|failed|deadletter"`) (RULE-OBS-01, RULE-OBS-03).

## 6. Data Model (indicative)
- `events(id, tenant_id, event_type, payload, dedup_key, created_at, created_by)`
  unique `(tenant_id, dedup_key)`; audit columns per RULE-DATA-03.
- `outbox(id, event_id, subscription_id, status, attempts, next_attempt_at, ...)`
- `subscriptions(id, tenant_id, event_type, target_url, created_at, ...)`

## 7. Acceptance Criteria
- AC-01: Ingesting the same `(tenant_id, dedup_key)` twice creates exactly one
  event row and returns the same event id (verifies FR-ING-03).
- AC-02: If the process is killed between event persist and delivery, the event
  is still delivered after restart (verifies FR-ING-02 outbox durability).
- AC-03: A subscriber returning 500 then 200 receives the event exactly via
  retry with backoff; a subscriber returning 400 is dead-lettered after N
  attempts (verifies FR-DEL-02).
- AC-04: A tenant cannot fetch another tenant's subscriptions (verifies
  FR-SUB-02 / RULE-SEC-01).
- AC-05: Race detector passes under concurrent ingestion + delivery load
  (verifies RULE-RES-02).
- AC-06: `/readyz` returns 503 when the DB is unreachable (verifies FR-OBS-01).

## 8. Constraints
- All invariants in docs/rules/domain_invariants.md are non-negotiable.
- PR sizing per docs/rules/spec_decomposition_rules.md (≤ 500 LOC per PR).
- Every FR/AC MUST be traceable through req → test → impl specs (RTM).
