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
- Go 1.23, 7-layer Clean Architecture (domain layer pure per RULE-ARCH-01/02).
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

## 9. Stage 1 requirements decomposition
Status: architecture and TestSpec approved by requester; gateway contract resolved. Stage 1 only; implementation requires a separate stage.
The preceding TRD is preserved verbatim. This addendum resolves compound requirements without replacing their IDs.
Reqspec triage: REQUIRED. Blast radius: HIGH. Database schema: yes; API contract: yes; multi-service: yes; critical state integrity: yes (delivery state, not financial processing).
Canonical architecture rules are RULE-ARCH-01, RULE-ARCH-02 and RULE-ARCH-03; the abbreviated reference in the original TRD does not omit either layer invariant.

| Atomic requirement | Parent | Acceptance contract (Given / When / Then) |
| --- | --- | --- |
| REQ-01 | FR-ING-01 | AC-07: Given authenticated tenant/actor context, when valid JSON is ingested, then return an event ID owned by that tenant; body/query identity cannot override it. |
| REQ-02 | FR-ING-02 | AC-02: Given matching subscriptions, when ingestion commits and the process stops before delivery, then restart eventually delivers every committed delivery; a failed transaction leaves neither event nor outbox rows. |
| REQ-03 | FR-ING-03 | AC-01: Given an existing tenant/dedup key, when repeated concurrently or after restart, then exactly one event exists and every successful response identifies it. |
| REQ-04 | FR-SUB-01 | AC-08: Given authenticated ownership, when creating a valid subscription, then persist its event type and target URL with audit metadata. |
| REQ-05 | FR-SUB-01 | AC-09: Given subscriptions across tenants, when listing, then return only the caller's active subscriptions in bounded pages. |
| REQ-06 | FR-SUB-01 | AC-10: Given an owned subscription, when deleting by ID, then soft-delete it and exclude it from future ingestion matching. |
| REQ-07 | FR-SUB-02 | AC-04: Given another tenant's subscription, when listing or deleting, then disclose no record and mutate no other tenant's state. |
| REQ-08 | FR-DEL-01 | AC-11: Given a pending delivery, when dispatched, then POST to its target with propagated cancellation, deadline and trace metadata. |
| REQ-09 | FR-DEL-02 | AC-12: Given a claimed delivery, when a 2xx response arrives, then persist delivered state and close the response body. |
| REQ-10 | FR-DEL-02 | AC-03: Given 500 then 200 responses, when the injected clock advances through backoff, then retry and finish delivered; given repeated 400, dead-letter at N completed attempts. |
| REQ-11 | FR-DEL-03 | AC-05: Given concurrent ingestion and delivery, when cancellation occurs, then worker count stays bounded, all workers terminate and the race detector reports no races. |
| REQ-12 | FR-DEL-04 | AC-13: Given retries or recovered claims, when a delivery is resent, then every attempt carries the same X-Relaybox-Delivery-Id; distinct subscriber deliveries have distinct IDs. |
| REQ-13 | FR-OBS-01 | AC-14: Given a running process, when /livez is called, then return 200 independently of DB availability. |
| REQ-14 | FR-OBS-01 | AC-06: Given an unreachable DB, when /readyz is called, then return 503 within its deadline; a reachable DB returns 200. |
| REQ-15 | FR-OBS-02 | AC-15: Given an operation with trace context, when logging, then emit context-bound JSON with trace/span IDs and no secrets or payloads. |
| REQ-16 | FR-OBS-02 | AC-16: Given requests and delivery outcomes, when metrics are observed, then histograms/error counters and outcome counts use finite label sets only. |

## 10. Error and protocol contract
| Sentinel | Code | HTTP | gRPC | Meaning |
| --- | --- | --- | --- | --- |
| ErrUnauthenticated | UNAUTHENTICATED | 401 | N/A | Missing or invalid trusted identity |
| ErrInvalidInput | INVALID_INPUT | 400 | N/A | Invalid JSON, required field or target URL |
| ErrNotFound | NOT_FOUND | 404 | N/A | Missing or inaccessible tenant-owned resource |
| ErrConflict | CONFLICT | 409 | N/A | Invalid concurrent state transition; duplicate ingestion instead returns original ID |
| ErrUnavailable | UNAVAILABLE | 503 | N/A | Unavailable persistence/dependency; no raw driver details |
| ErrInternal | INTERNAL | 500 | N/A | Unexpected internal failure with sanitized response |
Cancellation terminates work; do not fabricate a response after a client disconnect. A retryable ingestion error never establishes whether an uncertain commit occurred: callers reuse the same dedup key.

## 11. Invariant coverage obligations
| Rules | Required proof in downstream testspec |
| --- | --- |
| RULE-ARCH-01, RULE-ARCH-02, RULE-ARCH-03 | Pure domain imports, inward dependency graph, typed errors and sanitized transport mapping |
| RULE-DATA-01, RULE-DATA-02, RULE-DATA-03 | Atomic event/fan-out writes, deduplication and stable delivery IDs, UTC creation/update/deletion actor metadata on all tables |
| RULE-RES-01, RULE-RES-02, RULE-RES-03 | Context/deadline propagation, bounded cancellation-aware workers, immediate cleanup binding for bodies/rows/transactions |
| RULE-SEC-01, RULE-SEC-02 | Authenticated context identity, tenant-scoped reads/writes and joins, telemetry redaction |
| RULE-EVT-02 | Commit-before-remote-delivery and recoverable durable outbox |
| RULE-OBS-01, RULE-OBS-02, RULE-OBS-03, RULE-OBS-04 | Context JSON logs, HTTP/SQL trace propagation, finite metrics and truthful health probes |
RULE-EVT-01 is non-applicable: there is no broker consumer or Ack/Nack interface. Durable retry/dead-letter semantics remain required by FR-DEL-02.
All applicable obligations map through TestSpec to the execution manifest; RULE-EVT-01 remains explicitly non-applicable.

## 12. Approved assumptions
ASSUMP-01: Gateway authentication follows the approved contract below.
ASSUMP-02: Subscription membership and target URL are snapshotted at ingestion; no retrospective delivery to later subscriptions. Unmatched events are retained.
ASSUMP-03: Soft deletion preserves already queued deliveries; cancellation of queued deliveries is not part of this feature.
ASSUMP-04: At-least-once does not guarantee subscriber exactly-once effects; receivers deduplicate stable delivery IDs.
ASSUMP-05: RFC defaults, response classifications and URL safety are approved architecture decisions.
Out of scope remains the TRD non-goals plus update endpoints, broker integration and Stage 2 implementation in this PR.

## 13. Approved gateway contract
REQ-17 (FR-ING-01, FR-SUB-02), AC-17: Given a configured gateway secret, when any business endpoint receives a request, then require X-Internal-Secret matching GATEWAY_SHARED_SECRET and one X-Tenant-ID containing a valid UUID; otherwise return 401 before any persistence call.
REQ-18 (FR-ING-01), AC-18: Given authenticated gateway headers, when context is constructed, then store typed tenant identity and optional X-Actor-ID in context; body/query identity never overrides it.
Empty configured secret prevents startup; missing, duplicate or invalid authentication headers fail closed. Compare secret digests in constant time. Never forward gateway headers to subscribers or include them in telemetry.
Optional actor absence uses the explicit audit principal gateway-system; background mutations use relaybox-dispatcher. These are service identities, not claims of a human actor.
ASSUMP-06: Gateway strips client-supplied identity/secret headers and injects verified values; TLS and network access restrictions protect the shared secret. Possession of this shared secret authenticates the gateway, not each end user independently.
NFR-01: Worker count <= configured W (default 8); HTTP timeout 10s, lease 30s, graceful shutdown 15s; validate lease exceeds timeout plus persistence margin.
NFR-02: Domain/service coverage >=85%; every sentinel branch has an asserted test; synchronization uses injected time, channels and deadlines, never sleeps.
SC-01: All 11 original FRs and all six original ACs have executable test targets and implementation owners; no implementation is claimed by this spec PR.
