# ReqSpec: Relaybox — Multi-Tenant Webhook Relay Service

This document captures the detailed functional and non-functional requirements for the Relaybox service, derived directly from the `relaybox-trd`.

## 1. Functional Requirements

### FR-ING-01: Event Ingestion Endpoint
- **Description:** Expose an HTTP POST endpoint for event ingestion.
- **Endpoint:** `POST /v1/events`
- **Request Body (JSON):**
  ```json
  {
    "event_type": "string",
    "payload": {}, // Arbitrary JSON payload
    "dedup_key": "string" // Optional, for idempotency
  }
  ```
- **Constraint:** Tenant identity MUST be resolved from the authenticated request context, never from the request body (RULE-SEC-01).

### FR-ING-02: Transactional Event Persistence
- **Description:** Event persistence and enqueuing for delivery must be atomic.
- **Constraint:** Persisting an event and writing its entry to an outbox table MUST occur within a single database transaction (RULE-DATA-01). Remote publication (e.g., to a message broker) inside an open transaction is prohibited (RULE-EVT-02).

### FR-ING-03: Idempotent Ingestion
- **Description:** Ingestion operations must be idempotent.
- **Constraint:** Ingestion MUST be idempotent per `(tenant_id, dedup_key)` via a unique database constraint (RULE-DATA-02). Subsequent attempts with the same `(tenant_id, dedup_key)` should return the original event ID without creating a new event.

### FR-SUB-01: Subscription Management Endpoints
- **Description:** Expose CRUD (Create, Read, Update, Delete) HTTP endpoints for managing event subscriptions.
- **Endpoints:** `POST /v1/subscriptions`, `GET /v1/subscriptions/{id}`, `GET /v1/subscriptions`, `DELETE /v1/subscriptions/{id}`
- **Request/Response:** Define structures mapping an `event_type` to a `target_url` for the authenticated tenant.

### FR-SUB-02: Tenant-Scoped Subscriptions
- **Description:** Subscriptions must be strictly isolated by tenant.
- **Constraint:** Subscriptions are scoped strictly to the owning tenant; a tenant can never read or mutate another tenant's subscriptions (RULE-SEC-01).

### FR-DEL-01: Background Dispatcher with Context Propagation
- **Description:** A background worker must read pending outbox rows and dispatch HTTP POST requests.
- **Constraint:** All outbound HTTP calls MUST propagate `ctx context.Context` and OpenTelemetry trace metadata (RULE-RES-01, RULE-OBS-02). A timeout must be set for outbound calls. The dispatcher MUST NOT use `context.Background()` or `context.TODO()` (RULE-RES-01).

### FR-DEL-02: At-Least-Once Delivery with Retry & Dead-Letter
- **Description:** Event delivery to external endpoints must be at-least-once with robust error handling.
- **Behavior:**
    - A `2xx` HTTP response from the subscriber marks the outbox row as delivered.
    - A transient failure (e.g., HTTP timeout, `5xx` status code) is retried with exponential backoff.
    - A permanent failure (e.g., `4xx` status code) is dead-lettered after N configurable attempts.
- **Constraint:** HTTP response bodies for outbound calls MUST be closed immediately (`defer resp.Body.Close()`) (RULE-RES-03).

### FR-DEL-03: Bounded Concurrency for Dispatcher
- **Description:** The background dispatcher must operate with bounded concurrency.
- **Constraint:** The dispatcher MUST use a bounded worker pool tied to context cancellation and graceful shutdown. Unbounded goroutines are strictly prohibited (RULE-RES-02).

### FR-DEL-04: Idempotent Delivery Attempts
- **Description:** Each delivery attempt to a subscriber must be idempotent from the subscriber's perspective.
- **Mechanism:** Each attempt MUST send a stable `X-Relaybox-Delivery-Id` header (RULE-DATA-02) to allow the subscriber to implement its own idempotency.

### FR-OBS-01: Health Endpoints
- **Description:** Expose standard health check endpoints.
- **Endpoints:**
    - `/livez`: Liveness probe (process alive).
    - `/readyz`: Readiness probe (checks database reachability) (RULE-OBS-04).

### FR-OBS-02: Structured Logging & Prometheus Metrics
- **Description:** Implement production-grade observability.
- **Logging:** Emit structured JSON logs bound to `ctx context.Context` (RULE-OBS-01).
- **Metrics:** Expose Prometheus metrics for request duration and delivery outcomes.
- **Constraint:** Metric labels for delivery outcomes (e.g., `status="delivered|failed|deadletter"`) MUST use bounded-cardinality labels to prevent time-series database memory exhaustion (RULE-OBS-03).

## 2. Data Model (Canonical)

### `events` table
- `id` (UUID, PK)
- `tenant_id` (UUID)
- `event_type` (TEXT)
- `payload` (JSONB)
- `dedup_key` (TEXT, unique with tenant_id)
- `created_at` (TIMESTAMPTZ, audit)
- `created_by` (TEXT, audit)
- `updated_at` (TIMESTAMPTZ, audit)
- `updated_by` (TEXT, audit)
- **Unique Constraint:** `(tenant_id, dedup_key)` for `FR-ING-03`.
- **Audit Columns:** Per `RULE-DATA-03`.

### `outbox` table
- `id` (UUID, PK)
- `event_id` (UUID, FK to `events.id`)
- `subscription_id` (UUID, FK to `subscriptions.id`)
- `status` (TEXT, e.g., 'PENDING', 'DELIVERED', 'FAILED', 'DEAD_LETTERED')
- `attempts` (INT)
- `next_attempt_at` (TIMESTAMPTZ)
- `last_attempt_at` (TIMESTAMPTZ)
- `error_message` (TEXT, nullable)
- `DeliveryTraceID`  (TEXT, For FR-DEL-04, X-Relaybox-Delivery-Id)
- `created_at` (TIMESTAMPTZ, audit)
- `created_by` (TEXT, audit)
- `updated_at` (TIMESTAMPTZ, audit)
- `updated_by` (TEXT, audit)
- **Audit Columns:** Per `RULE-DATA-03`.

### `subscriptions` table
- `id` (UUID, PK)
- `tenant_id` (UUID)
- `event_type` (TEXT)
- `target_url` (TEXT)
- `created_at` (TIMESTAMPTZ, audit)
- `created_by` (TEXT, audit)
- `updated_at` (TIMESTAMPTZ, audit)
- `updated_by` (TEXT, audit)
- **Audit Columns:** Per `RULE-DATA-03`.

## 3. Acceptance Criteria

- **AC-01 (Verifies FR-ING-03):** Ingesting the same `(tenant_id, dedup_key)` twice creates exactly one event row in the `events` table and returns the same unique event ID for both requests.
- **AC-02 (Verifies FR-ING-02):** If the Relaybox process is killed between the event being persisted and its corresponding entry being added to the outbox, the event is still reliably delivered after the process restarts and the dispatcher resumes operation.
- **AC-03 (Verifies FR-DEL-02):**
    - A subscriber endpoint returning an HTTP `500` status code for the first attempt, then an `HTTP 200` for a subsequent attempt, receives the event exactly once via the retry mechanism with exponential backoff.
    - A subscriber endpoint consistently returning `HTTP 400` status codes is moved to a dead-letter state after N (e.g., 5) configured attempts, and no further delivery attempts are made for that specific event and subscription.
- **AC-04 (Verifies FR-SUB-02 / RULE-SEC-01):** An authenticated tenant attempting to fetch, update, or delete a subscription belonging to a different tenant receives an authorization error (e.g., HTTP 403 Forbidden or 404 Not Found), without exposing any information about the other tenant's subscription.
- **AC-05 (Verifies RULE-RES-02):** Running `go test -race` under a simulated concurrent ingestion and delivery load (e.g., multiple clients ingesting events and the dispatcher actively processing the outbox) passes without reporting any data races.
- **AC-06 (Verifies FR-OBS-01 / RULE-OBS-04):** The `/readyz` health endpoint returns an HTTP `503 Service Unavailable` status code and an appropriate error message when the configured PostgreSQL database is unreachable, and returns `HTTP 200 OK` when the database connection is healthy.

## 4. Blast Radius Assessment

| Blast Radius Criterion | Evaluation Question | Status |
| :--------------------- | :-------------------------------------------------------------- | :----- |
| **Database Schema** | Does this add/alter tables, modify columns, or require migrations? | Yes |
| **API Contract** | Does this alter public request/response payloads or break backward compatibility? | Yes |
| **Multi-Service Boundary** | Does this affect cross-service RPCs, shared message brokers, or third-party APIs? | Yes |
| **Data / Financial Integrity** | Does this mutate ledger balances, financial records, or critical state transitions? | Yes |

**Classification: HIGH** (Mandates an Architecture RFC and approval at RFC Approval Gate)