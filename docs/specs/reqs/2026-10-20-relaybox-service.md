# ReqSpec: Relaybox - Multi-Tenant Webhook Relay Service Requirements

## 1. Feature Overview
Relaybox is a new standalone Go service designed to reliably ingest, persist, and deliver domain events to external subscriber HTTP endpoints. It provides durable, at-least-once delivery with retries, idempotency, and strict multi-tenant isolation.

## 2. Blast Radius Assessment
| Blast Radius Criterion | Evaluation Question | Status |
| :--- | :--- | :--- |
| **Database Schema** | Does this add/alter tables, modify columns, or require migrations? | Yes |
| **API Contract** | Does this alter public request/response payloads or break backward compatibility? | Yes |
| **Multi-Service Boundary** | Does this affect cross-service RPCs, shared message brokers, or third-party APIs? | Yes |
| **Data / Financial Integrity** | Does this mutate ledger balances, financial records, or critical state transitions? | Yes |

**Classification: HIGH** (Requires RFC Approval Gate due to multiple 'Yes' answers, impacting DB schema, API, cross-service interactions, and data integrity.)

## 3. Functional Requirements (FR-xx)

### Ingestion
- **FR-ING-01**: Expose `POST /v1/events` accepting a JSON event `{ event_type, payload, dedup_key }`. Tenant identity MUST be resolved from the authenticated request context, never the body (RULE-SEC-01).
- **FR-ING-02**: Persisting an event and enqueuing it for delivery MUST occur in a single database transaction (RULE-DATA-01), writing to an outbox table (RULE-EVT-02). No remote publish inside the open transaction.
- **FR-ING-03**: Ingestion MUST be idempotent per `(tenant_id, dedup_key)` via a unique constraint (RULE-DATA-02). Re-sends return the original event id.

### Subscriptions
- **FR-SUB-01**: Expose CRUD `POST/GET/DELETE /v1/subscriptions` mapping an `event_type` to a target URL for the authenticated tenant.
- **FR-SUB-02**: Subscriptions are scoped strictly to the owning tenant; a tenant can never read or mutate another tenant's subscriptions (RULE-SEC-01).

### Delivery (external endpoint access)
- **FR-DEL-01**: A background dispatcher reads pending outbox rows and sends an HTTP POST to each matching subscriber URL. All outbound calls MUST propagate `ctx` and OpenTelemetry trace metadata (RULE-RES-01, RULE-OBS-02) and set a timeout; `context.Background()` is forbidden in the dispatcher (RULE-RES-01).
- **FR-DEL-02**: Delivery is at-least-once. A 2xx response marks the row delivered; a transient failure (timeout, 5xx) is retried with exponential backoff; a permanent failure (4xx) is dead-lettered after N attempts. Response bodies MUST be closed immediately (RULE-RES-03).
- **FR-DEL-03**: The dispatcher MUST use a bounded worker pool tied to context cancellation and graceful shutdown; no unbounded goroutines (RULE-RES-02).
- **FR-DEL-04**: Delivery attempts MUST be idempotent from the subscriber's view: each attempt sends a stable `X-Relaybox-Delivery-Id` header (RULE-DATA-02).

### Observability & Health
- **FR-OBS-01**: Expose `/livez` and `/readyz`; `/readyz` checks DB reachability (RULE-OBS-04).
- **FR-OBS-02**: Emit structured JSON logs bound to ctx, and Prometheus metrics for request duration and delivery outcomes with bounded-cardinality labels (`status="delivered|failed|deadletter"`) (RULE-OBS-01, RULE-OBS-03).

## 4. Non-Functional Requirements (NFR-xx)
- **NFR-01**: `p99` latency for `POST /v1/events` must be below 50ms at 1000 RPS.
- **NFR-02**: The system must achieve at-least-once delivery for 100% of events under normal operating conditions.
- **NFR-03**: Strict tenant isolation (RULE-SEC-01) must be enforced across all API and data access paths.

## 5. Acceptance Criteria (AC-xx)
- **AC-01**: Ingesting the same `(tenant_id, dedup_key)` twice creates exactly one event row and returns the same event id (verifies FR-ING-03).
- **AC-02**: If the process is killed between event persist and delivery, the event is still delivered after restart (verifies FR-ING-02 outbox durability).
- **AC-03**: A subscriber returning 500 then 200 receives the event exactly via retry with backoff; a subscriber returning 400 is dead-lettered after N attempts (verifies FR-DEL-02).
- **AC-04**: A tenant cannot fetch another tenant's subscriptions (verifies FR-SUB-02 / RULE-SEC-01).
- **AC-05**: Race detector passes under concurrent ingestion + delivery load (verifies RULE-RES-02).
- **AC-06**: `/readyz` returns 503 when the DB is unreachable (verifies FR-OBS-01).

## 6. Error Catalog & Protocol Mapping
| Error Sentinel | Error Code String | HTTP Status | gRPC Status | Description / Client Message |
| :--- | :--- | :--- | :--- | :--- |
| `ErrInvalidEventPayload` | `RELAYBOX_INVALID_EVENT_PAYLOAD` | 400 Bad Request | InvalidArgument | The provided event payload is malformed or invalid. |
| `ErrDuplicateEvent` | `RELAYBOX_DUPLICATE_EVENT` | 409 Conflict | AlreadyExists | An event with the same dedup_key already exists for this tenant. |
| `ErrSubscriptionNotFound` | `RELAYBOX_SUBSCRIPTION_NOT_FOUND` | 404 Not Found | NotFound | The requested subscription could not be found. |
| `ErrUnauthorized` | `RELAYBOX_UNAUTHORIZED` | 403 Forbidden | PermissionDenied | The authenticated tenant lacks permission for this resource. |
| `ErrInternal` | `RELAYBOX_INTERNAL_ERROR` | 500 Internal Server Error | Internal | An unexpected internal server error occurred. |
